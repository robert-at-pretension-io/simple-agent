# Unified Diff Coder Logic and Techniques

This document details the implementation of the Unified Diff (`udiff`) coder in Aider. It covers the prompting strategy used to guide the LLM, the parsing logic to extract edits, and the application logic (including fuzzy matching techniques) used to apply those edits to the codebase.

## 1. Prompting Strategy

The `udiff` coder instructs the LLM to generate changes in a format similar to `diff -U0`, but simplified for easier generation and parsing.

### System Prompts

The main system prompt sets the persona and basic instructions.

**Source:** `aider/coders/udiff_prompts.py`

```python
    main_system = """Act as an expert software developer.
{final_reminders}
Always use best practices when coding.
Respect and use existing conventions, libraries, etc that are already present in the code base.

Take requests for changes to the supplied code.
If the request is ambiguous, ask questions.

For each file that needs to be changed, write out the changes similar to a unified diff like `diff -U0` would produce.
"""
```

### Detailed Formatting Rules

The `system_reminder` provides specific constraints to ensure the LLM generates valid diffs. Key techniques include:
1.  **No Line Numbers:** Removing line numbers reduces hallucination errors.
2.  **Context:** Using `@@ ... @@` as a separator.
3.  **Whole Block Replacement:** Encouraging the replacement of entire functions/blocks rather than small internal lines to ensure context uniqueness and correctness.

**Source:** `aider/coders/udiff_prompts.py`

```python
    system_reminder = """# File editing rules:

Return edits similar to unified diffs that `diff -U0` would produce.

Make sure you include the first 2 lines with the file paths.
Don't include timestamps with the file paths.

Start each hunk of changes with a `@@ ... @@` line.
Don't include line numbers like `diff -U0` does.
The user's patch tool doesn't need them.

The user's patch tool needs CORRECT patches that apply cleanly against the current contents of the file!
Think carefully and make sure you include and mark all lines that need to be removed or changed as `-` lines.
Make sure you mark all new or modified lines with `+`.
Don't leave out any lines or the diff patch won't apply correctly.

Indentation matters in the diffs!

Start a new hunk for each section of the file that needs changes.

Only output hunks that specify changes with `+` or `-` lines.
Skip any hunks that are entirely unchanging ` ` lines.

Output hunks in whatever order makes the most sense.
Hunks don't need to be in any particular order.

When editing a function, method, loop, etc use a hunk to replace the *entire* code block.
Delete the entire existing version with `-` lines and then add a new, updated version with `+` lines.
This will help you generate correct code and correct diffs.

To move code within a file, use 2 hunks: 1 to delete it from its current location, 1 to insert it in the new location.

To make a new file, show a diff from `--- /dev/null` to `+++ path/to/new/file.ext`.

{final_reminders}
"""
```

### Example Interaction

The prompts include examples to demonstrate the expected format.

**Source:** `aider/coders/udiff_prompts.py`

```python
    example_messages = [
        dict(
            role="user",
            content="Replace is_prime with a call to sympy.",
        ),
        dict(
            role="assistant",
            content="""Ok, I will:

1. Add an imports of sympy.
2. Remove the is_prime() function.
3. Replace the existing call to is_prime() with a call to sympy.isprime().

Here are the diffs for those changes:

```diff
--- mathweb/flask/app.py
+++ mathweb/flask/app.py
@@ ... @@
-class MathWeb:
+import sympy
+
+class MathWeb:
@@ ... @@
-def is_prime(x):
-    if x < 2:
-        return False
-    for i in range(2, int(math.sqrt(x)) + 1):
-        if x % i == 0:
-            return False
-    return True
@@ ... @@
-@app.route('/prime/<int:n>')
-def nth_prime(n):
-    count = 0
-    num = 1
-    while count < n:
-        num += 1
-        if is_prime(num):
-            count += 1
-    return str(num)
+@app.route('/prime/<int:n>')
+def nth_prime(n):
+    count = 0
+    num = 1
+    while count < n:
+        num += 1
+        if sympy.isprime(num):
+            count += 1
+    return str(num)
```
""",
        ),
    ]
```

## 2. Parsing Logic

The coder parses the LLM's response to extract diff blocks. It looks for fenced code blocks marked with `diff` and parses the standard `---`/`+++` headers.

**Source:** `aider/coders/udiff_coder.py`

```python
def find_diffs(content):
    # We can always fence with triple-quotes, because all the udiff content
    # is prefixed with +/-/space.

    if not content.endswith("\n"):
        content = content + "\n"

    lines = content.splitlines(keepends=True)
    line_num = 0
    edits = []
    while line_num < len(lines):
        while line_num < len(lines):
            line = lines[line_num]
            if line.startswith("```diff"):
                line_num, these_edits = process_fenced_block(lines, line_num + 1)
                edits += these_edits
                break
            line_num += 1

    # For now, just take 1!
    # edits = edits[:1]

    return edits
```

The `process_fenced_block` function handles the extraction of filenames and hunks from within the fenced block.

```python
def process_fenced_block(lines, start_line_num):
    # ... (logic to find end of block) ...

    if block[0].startswith("--- ") and block[1].startswith("+++ "):
        # Extract the file path, considering that it might contain spaces
        a_fname = block[0][4:].strip()
        b_fname = block[1][4:].strip()

        # Check if standard git diff prefixes are present (or /dev/null) and strip them
        if (a_fname.startswith("a/") or a_fname == "/dev/null") and b_fname.startswith("b/"):
            fname = b_fname[2:]
        else:
            # Otherwise, assume the path is as intended
            fname = b_fname

        block = block[2:]
    else:
        fname = None

    # ... (logic to parse +/- lines into hunks) ...
```

## 3. Application Logic and Fuzzy Matching

The core complexity lies in applying the diffs. Since the LLM might generate slightly inexact context or whitespace, the coder employs several strategies to apply the changes.

### Hunk Normalization

Before application, hunks are normalized to handle whitespace inconsistencies.

**Source:** `aider/coders/udiff_coder.py`

```python
def normalize_hunk(hunk):
    before, after = hunk_to_before_after(hunk, lines=True)

    before = cleanup_pure_whitespace_lines(before)
    after = cleanup_pure_whitespace_lines(after)

    diff = difflib.unified_diff(before, after, n=max(len(before), len(after)))
    diff = list(diff)[3:]
    return diff
```

### Application Strategy

The `apply_hunk` function attempts to apply changes using a tiered approach:
1.  **Direct Application:** Exact match search and replace.
2.  **Context Adjustment:** `make_new_lines_explicit` attempts to re-diff the hunk against the current file content to adjust for minor context differences.
3.  **Partial Application:** `apply_partial_hunk` attempts to apply the hunk by reducing the required context (preceding/following lines) if the full context doesn't match.

**Source:** `aider/coders/udiff_coder.py`

```python
def apply_hunk(content, hunk):
    before_text, after_text = hunk_to_before_after(hunk)

    res = directly_apply_hunk(content, hunk)
    if res:
        return res

    hunk = make_new_lines_explicit(content, hunk)

    # just consider space vs not-space
    ops = "".join([line[0] for line in hunk])
    ops = ops.replace("-", "x")
    ops = ops.replace("+", "x")
    ops = ops.replace("\n", " ")

    cur_op = " "
    section = []
    sections = []

    # ... (logic to split hunk into sections) ...

    all_done = True
    for i in range(2, len(sections), 2):
        preceding_context = sections[i - 2]
        changes = sections[i - 1]
        following_context = sections[i]

        res = apply_partial_hunk(content, preceding_context, changes, following_context)
        if res:
            content = res
        else:
            all_done = False
            # FAILED!
            # this_hunk = preceding_context + changes + following_context
            break

    if all_done:
        return content
```

### Direct Application

This uses a flexible search and replace mechanism (imported from `search_replace`).

```python
def directly_apply_hunk(content, hunk):
    before, after = hunk_to_before_after(hunk)

    if not before:
        return

    before_lines, _ = hunk_to_before_after(hunk, lines=True)
    before_lines = "".join([line.strip() for line in before_lines])

    # Refuse to do a repeated search and replace on a tiny bit of non-whitespace context
    if len(before_lines) < 10 and content.count(before) > 1:
        return

    try:
        new_content = flexi_just_search_and_replace([before, after, content])
    except SearchTextNotUnique:
        new_content = None

    return new_content
```

### Partial Application (Fuzzy Matching)

This function tries to apply the change by iteratively reducing the amount of required context (lines before and after the change) until a match is found or the context is exhausted.

```python
def apply_partial_hunk(content, preceding_context, changes, following_context):
    len_prec = len(preceding_context)
    len_foll = len(following_context)

    use_all = len_prec + len_foll

    # if there is a - in the hunk, we can go all the way to `use=0`
    for drop in range(use_all + 1):
        use = use_all - drop

        for use_prec in range(len_prec, -1, -1):
            if use_prec > use:
                continue

            use_foll = use - use_prec
            if use_foll > len_foll:
                continue

            if use_prec:
                this_prec = preceding_context[-use_prec:]
            else:
                this_prec = []

            this_foll = following_context[:use_foll]

            res = directly_apply_hunk(content, this_prec + changes + this_foll)
            if res:
                return res
```

## 4. Error Handling

If the diffs cannot be applied, specific error messages are generated to guide the user (and potentially the LLM in a retry loop) on why the patch failed.

**Source:** `aider/coders/udiff_coder.py`

```python
no_match_error = """UnifiedDiffNoMatch: hunk failed to apply!

{path} does not contain lines that match the diff you provided!
Try again.
DO NOT skip blank lines, comments, docstrings, etc!
The diff needs to apply cleanly to the lines in {path}!

{path} does not contain these {num_lines} exact lines in a row:
```
{original}```
"""


not_unique_error = """UnifiedDiffNotUnique: hunk failed to apply!

{path} contains multiple sets of lines that match the diff you provided!
Try again.
Use additional ` ` lines to provide context that uniquely indicates which code needs to be changed.
The diff needs to apply to a unique set of lines in {path}!

{path} contains multiple copies of these {num_lines} lines:
```
{original}```
"""
```
