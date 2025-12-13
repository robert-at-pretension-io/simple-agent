#!/usr/bin/env python3
"""
PDF to Gemini OCR Script
Converts PDF to images and processes each page through Gemini API (OpenAI compatibility layer)
"""

import os
import sys
import base64
import argparse
import subprocess
import tempfile
import shutil
import hashlib
import time
import re
import random
from pathlib import Path
from datetime import datetime
from concurrent.futures import ThreadPoolExecutor, as_completed
from openai import OpenAI


# ANSI color codes
class Colors:
    RED = '\033[0;31m'
    GREEN = '\033[0;32m'
    YELLOW = '\033[1;33m'
    BLUE = '\033[0;34m'
    NC = '\033[0m'  # No Color


class Logger:
    """Handles logging to both console and file"""

    def __init__(self, log_file=None):
        self.log_file = log_file
        self.log_handle = None
        if log_file:
            self.log_handle = open(log_file, 'w', encoding='utf-8')
            self.log(f"Log started at {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")

    def _strip_ansi(self, text):
        """Remove ANSI color codes from text"""
        ansi_escape = re.compile(r'\x1B(?:[@-Z\\-_]|\[[0-?]*[ -/]*[@-~])')
        return ansi_escape.sub('', text)

    def log(self, message, end='\n'):
        """Log message to console and file"""
        # Print to console with colors
        print(message, end=end)

        # Write to file without colors
        if self.log_handle:
            clean_message = self._strip_ansi(message)
            self.log_handle.write(clean_message + end)
            self.log_handle.flush()

    def close(self):
        """Close the log file"""
        if self.log_handle:
            self.log(f"\nLog ended at {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
            self.log_handle.close()
            self.log_handle = None


class CacheManager:
    """Manages caching of OCR results"""

    def __init__(self, cache_dir=".ocr_cache"):
        self.cache_dir = Path(cache_dir)

    def init_cache(self):
        """Initialize cache directory"""
        if not self.cache_dir.exists():
            self.cache_dir.mkdir(parents=True)
            print(f"{Colors.BLUE}Created cache directory: {self.cache_dir}{Colors.NC}")

    def get_cache_key(self, pdf_path):
        """Generate cache key based on PDF filename and modification time"""
        pdf_path = Path(pdf_path)
        pdf_name = pdf_path.name
        pdf_mtime = int(pdf_path.stat().st_mtime)
        cache_string = f"{pdf_name}_{pdf_mtime}"
        return hashlib.md5(cache_string.encode()).hexdigest()

    def get_cache_file(self, pdf_path):
        """Get cache metadata file path"""
        cache_key = self.get_cache_key(pdf_path)
        return self.cache_dir / f"{cache_key}.cache"

    def get_content_file(self, pdf_path, page_num):
        """Get cache content file path for a specific page"""
        cache_key = self.get_cache_key(pdf_path)
        return self.cache_dir / f"{cache_key}_page{page_num}.txt"

    def is_page_cached(self, pdf_path, page_num):
        """Check if a page is cached"""
        cache_file = self.get_cache_file(pdf_path)
        if not cache_file.exists():
            return False

        with open(cache_file, 'r') as f:
            cached_pages = f.read().splitlines()

        return str(page_num) in cached_pages

    def mark_page_cached(self, pdf_path, page_num):
        """Mark a page as cached"""
        if not self.is_page_cached(pdf_path, page_num):
            cache_file = self.get_cache_file(pdf_path)
            with open(cache_file, 'a') as f:
                f.write(f"{page_num}\n")

    def save_page_content(self, pdf_path, page_num, content):
        """Save page OCR content to cache"""
        content_file = self.get_content_file(pdf_path, page_num)
        with open(content_file, 'w', encoding='utf-8') as f:
            f.write(content)

    def load_page_content(self, pdf_path, page_num):
        """Load page OCR content from cache"""
        content_file = self.get_content_file(pdf_path, page_num)
        if content_file.exists():
            with open(content_file, 'r', encoding='utf-8') as f:
                return f.read()
        return None

    def get_cache_stats(self, pdf_path):
        """Get number of cached pages"""
        cache_file = self.get_cache_file(pdf_path)
        if not cache_file.exists():
            return 0

        with open(cache_file, 'r') as f:
            return len(f.read().splitlines())

    def clear_cache(self, pdf_path=None):
        """Clear cache for specific PDF or all cache"""
        if pdf_path:
            cache_key = self.get_cache_key(pdf_path)
            # Remove all files with this cache key
            for file in self.cache_dir.glob(f"{cache_key}*"):
                file.unlink()
            print(f"{Colors.GREEN}Cache cleared for: {pdf_path}{Colors.NC}")
        else:
            if self.cache_dir.exists():
                shutil.rmtree(self.cache_dir)
                print(f"{Colors.GREEN}All cache cleared{Colors.NC}")
            else:
                print(f"{Colors.YELLOW}No cache directory found{Colors.NC}")


class PDFOCRProcessor:
    """Main processor for PDF OCR using Gemini"""

    def __init__(self, api_key, logger=None):
        self.client = OpenAI(
            api_key=api_key,
            base_url="https://generativelanguage.googleapis.com/v1beta/openai/"
        )
        self.model = "gemini-3-pro-preview"
        self.logger = logger if logger else Logger()  # Use provided logger or create basic one

    def check_dependencies(self):
        """Check if required tools are installed"""
        missing = []

        if not shutil.which('pdftoppm'):
            missing.append('pdftoppm (poppler-utils)')

        if missing:
            self.logger.log(f"{Colors.RED}Error: Missing required dependencies:{Colors.NC}")
            for dep in missing:
                self.logger.log(f"  - {dep}")
            self.logger.log("\nInstall on Ubuntu/Debian: sudo apt-get install poppler-utils")
            self.logger.log("Install on macOS: brew install poppler")
            sys.exit(1)

    def convert_pdf_to_images(self, pdf_path, temp_dir, dpi=300):
        """Convert PDF to PNG images"""
        self.logger.log(f"{Colors.YELLOW}Converting PDF to images (DPI: {dpi})...{Colors.NC}")

        try:
            subprocess.run([
                'pdftoppm',
                '-png',
                '-r', str(dpi),
                str(pdf_path),
                str(Path(temp_dir) / 'page')
            ], check=True, capture_output=True)
        except subprocess.CalledProcessError as e:
            self.logger.log(f"{Colors.RED}Error converting PDF: {e.stderr.decode()}{Colors.NC}")
            sys.exit(1)

        # Count generated images
        images = list(Path(temp_dir).glob('page-*.png'))
        num_pages = len(images)
        self.logger.log(f"{Colors.GREEN}✓ Converted {num_pages} pages{Colors.NC}")

        return num_pages

    def _extract_retry_delay(self, error_message):
        """Extract retry delay from error message in seconds"""
        # Try to find "Please retry in X.XXXs" or "retryDelay': 'XXs'"
        patterns = [
            r'Please retry in ([\d.]+)s',
            r"'retryDelay':\s*'(\d+)s'"
        ]

        for pattern in patterns:
            match = re.search(pattern, str(error_message))
            if match:
                delay = float(match.group(1))
                return delay

        return None

    def _calculate_backoff(self, attempt, base_delay=2, max_delay=60):
        """Calculate exponential backoff with jitter"""
        # Exponential backoff: base_delay * 2^attempt
        delay = min(base_delay * (2 ** attempt), max_delay)
        # Add jitter (±25%)
        jitter = delay * 0.25 * (2 * random.random() - 1)
        return delay + jitter

    def process_image(self, image_path, prompt, page_num, context_text=None, max_retries=3):
        """Process a single image with Gemini API via OpenAI compatibility layer with retry logic"""
        if context_text:
            self.logger.log(f"{Colors.YELLOW}Processing page {page_num} (with context from {len(context_text)} previous page(s))...{Colors.NC}")
        else:
            self.logger.log(f"{Colors.YELLOW}Processing page {page_num}...{Colors.NC}")

        # Read and encode image
        with open(image_path, 'rb') as f:
            image_data = base64.b64encode(f.read()).decode('utf-8')

        # Build the prompt with context if available
        full_prompt = prompt
        if context_text:
            context_section = "CONTEXT FROM PREVIOUS PAGES:\n" + "="*40 + "\n\n"
            for page_num_ctx, text in context_text:
                context_section += f"--- Page {page_num_ctx} ---\n{text}\n\n"
            context_section += "="*40 + "\n\n"
            full_prompt = context_section + prompt

        # Retry logic
        for attempt in range(max_retries):
            try:
                response = self.client.chat.completions.create(
                    model=self.model,
                    messages=[
                        {
                            "role": "user",
                            "content": [
                                {
                                    "type": "text",
                                    "text": full_prompt
                                },
                                {
                                    "type": "image_url",
                                    "image_url": {
                                        "url": f"data:image/png;base64,{image_data}"
                                    }
                                }
                            ]
                        }
                    ],
                    temperature=1.0,
                    max_tokens=30000
                )

                # Extract content
                if response.choices and len(response.choices) > 0:
                    content = response.choices[0].message.content
                    if content and content.strip():  # Ensure non-empty content
                        self.logger.log(f"{Colors.GREEN}✓ Completed page {page_num}{Colors.NC}")
                        return content
                    else:
                        self.logger.log(f"{Colors.YELLOW}⚠ Empty content returned for page {page_num}{Colors.NC}")
                        if attempt < max_retries - 1:
                            backoff_delay = self._calculate_backoff(attempt)
                            self.logger.log(f"{Colors.YELLOW}  Retrying in {backoff_delay:.1f}s... (attempt {attempt + 2}/{max_retries}){Colors.NC}")
                            time.sleep(backoff_delay)
                            continue
                else:
                    self.logger.log(f"{Colors.YELLOW}⚠ No choices returned for page {page_num}{Colors.NC}")
                    if attempt < max_retries - 1:
                        backoff_delay = self._calculate_backoff(attempt)
                        self.logger.log(f"{Colors.YELLOW}  Retrying in {backoff_delay:.1f}s... (attempt {attempt + 2}/{max_retries}){Colors.NC}")
                        time.sleep(backoff_delay)
                        continue

            except Exception as e:
                error_str = str(e)
                is_rate_limit = "429" in error_str or "RESOURCE_EXHAUSTED" in error_str

                if is_rate_limit:
                    # Extract retry delay from error message
                    retry_delay = self._extract_retry_delay(error_str)

                    if retry_delay:
                        # Add positive jitter to the API-suggested delay (0-20% extra)
                        # Never wait LESS than the API suggests, only more to avoid thundering herd
                        jitter = retry_delay * 0.2 * random.random()
                        wait_time = retry_delay + jitter
                        self.logger.log(f"{Colors.YELLOW}⚠ Rate limit hit for page {page_num} (429){Colors.NC}")
                        if attempt < max_retries - 1:
                            self.logger.log(f"{Colors.YELLOW}  Waiting {wait_time:.1f}s before retry (API suggested {retry_delay:.1f}s)... (attempt {attempt + 2}/{max_retries}){Colors.NC}")
                            time.sleep(wait_time)
                            continue
                        else:
                            self.logger.log(f"{Colors.RED}✗ Failed page {page_num} after {max_retries} attempts (rate limit){Colors.NC}")
                            return None
                    else:
                        # Fallback to exponential backoff if we can't extract delay
                        backoff_delay = self._calculate_backoff(attempt, base_delay=5)
                        self.logger.log(f"{Colors.YELLOW}⚠ Rate limit hit for page {page_num} (429){Colors.NC}")
                        if attempt < max_retries - 1:
                            self.logger.log(f"{Colors.YELLOW}  Waiting {backoff_delay:.1f}s before retry... (attempt {attempt + 2}/{max_retries}){Colors.NC}")
                            time.sleep(backoff_delay)
                            continue
                        else:
                            self.logger.log(f"{Colors.RED}✗ Failed page {page_num} after {max_retries} attempts (rate limit){Colors.NC}")
                            return None
                else:
                    # Non-rate-limit error
                    self.logger.log(f"{Colors.YELLOW}⚠ Error processing page {page_num}: {e}{Colors.NC}")
                    if attempt < max_retries - 1:
                        backoff_delay = self._calculate_backoff(attempt)
                        self.logger.log(f"{Colors.YELLOW}  Retrying in {backoff_delay:.1f}s... (attempt {attempt + 2}/{max_retries}){Colors.NC}")
                        time.sleep(backoff_delay)
                        continue
                    else:
                        self.logger.log(f"{Colors.RED}✗ Failed page {page_num} after {max_retries} attempts{Colors.NC}")
                        return None

        # All retries exhausted
        self.logger.log(f"{Colors.RED}✗ Failed page {page_num} after {max_retries} attempts{Colors.NC}")
        return None

    def process_pdf(self, pdf_path, output_path, dpi=300, prompt=None,
                   use_cache=True, redo_pages=None, keep_images=False,
                   context_pages=0, start_page=1, num_pages=None,
                   max_retries=3, concurrent_workers=5):
        """Main processing function with concurrent and retry support"""

        self.logger.log(f"{Colors.GREEN}=== PDF to Gemini OCR Processing ==={Colors.NC}\n")

        # Initialize cache
        cache_manager = CacheManager()
        if use_cache:
            cache_manager.init_cache()

        # Set default prompt based on whether context is being used
        if prompt is None:
            if context_pages > 0:
                prompt = """Read this page image and transcribe ALL visible text exactly as written.

You are provided with context from the previous page(s) above to help maintain continuity and understanding of the document flow.

Extract the complete text content including book titles, author names, descriptions, links, and any other readable text.

IMPORTANT: Do NOT return just a list of numbers or indexes. Return the actual text content from the page.

Output the full text word-for-word in a readable format, taking into account the context from previous pages."""
            else:
                prompt = """Read this page image and transcribe ALL visible text exactly as written.

Extract the complete text content including book titles, author names, descriptions, links, and any other readable text.

IMPORTANT: Do NOT return just a list of numbers or indexes. Return the actual text content from the page.

Output the full text word-for-word in a readable format."""

        # Create temporary directory
        temp_dir = tempfile.mkdtemp()

        try:
            # Get cache statistics
            if use_cache:
                cached_count = cache_manager.get_cache_stats(pdf_path)
            else:
                cached_count = 0

            # Display redo pages
            if redo_pages:
                self.logger.log(f"{Colors.BLUE}Will reprocess pages: {','.join(map(str, redo_pages))}{Colors.NC}")

            # Display processing parameters
            self.logger.log(f"PDF File: {pdf_path}")
            self.logger.log(f"Output File: {output_path}")
            self.logger.log(f"Temporary Directory: {temp_dir}")
            if use_cache:
                self.logger.log(f"Cache: Enabled ({cached_count} pages cached)")
            else:
                self.logger.log(f"Cache: Disabled")
            if context_pages > 0:
                self.logger.log(f"Context: Using {context_pages} previous page(s) for context")
                self.logger.log(f"Processing: Sequential (required for context)")
            else:
                self.logger.log(f"Processing: Concurrent ({concurrent_workers} workers)")
            self.logger.log(f"Max Retries: {max_retries}")
            if start_page > 1 or num_pages is not None:
                end_page = start_page + num_pages - 1 if num_pages else "end"
                self.logger.log(f"Page Range: {start_page} to {end_page}")
            self.logger.log("")

            # Convert PDF to images
            total_pages = self.convert_pdf_to_images(pdf_path, temp_dir, dpi)

            if total_pages == 0:
                self.logger.log(f"{Colors.RED}Error: No pages were converted{Colors.NC}")
                return

            # Calculate actual end page
            if num_pages is not None:
                end_page = min(start_page + num_pages - 1, total_pages)
            else:
                end_page = total_pages

            # Validate start page
            if start_page > total_pages:
                self.logger.log(f"{Colors.RED}Error: --start-page ({start_page}) exceeds total pages ({total_pages}){Colors.NC}")
                return

            self.logger.log("")

            # Initialize output file
            with open(output_path, 'w', encoding='utf-8') as f:
                f.write("=" * 40 + "\n")
                f.write("Gemini OCR Processing Results\n")
                f.write("=" * 40 + "\n")
                f.write(f"Source: {pdf_path}\n")
                f.write(f"Date: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
                f.write(f"Total Pages in PDF: {total_pages}\n")
                f.write(f"Processing Pages: {start_page} to {end_page}\n")
                if context_pages > 0:
                    f.write(f"Context Pages: {context_pages}\n")
                f.write("=" * 40 + "\n\n")

            # Process each page
            success_count = 0
            skip_count = 0
            fail_count = 0

            # Track page content for context
            page_content = {}  # {page_num: content}

            # Get sorted list of image files
            image_files = sorted(Path(temp_dir).glob('page-*.png'))

            # Filter to pages in range
            pages_to_process = []
            for image_file in image_files:
                page_num = int(image_file.stem.split('-')[-1])
                if start_page <= page_num <= end_page:
                    pages_to_process.append((page_num, image_file))

            # Choose processing mode based on context
            if context_pages > 0:
                # Sequential processing (required for context)
                for page_num, image_file in pages_to_process:
                    # Check if page should be processed
                    should_process = True
                    result = None

                    if use_cache:
                        # Check if page is in redo list
                        if redo_pages and page_num in redo_pages:
                            self.logger.log(f"{Colors.BLUE}Reprocessing page {page_num} (requested){Colors.NC}")
                            should_process = True
                        elif cache_manager.is_page_cached(pdf_path, page_num):
                            self.logger.log(f"{Colors.BLUE}⊙ Loading page {page_num} from cache{Colors.NC}")
                            should_process = False
                            skip_count += 1

                            # Load cached content
                            cached_content = cache_manager.load_page_content(pdf_path, page_num)
                            if cached_content:
                                result = cached_content
                                # Append cached content to output
                                with open(output_path, 'a', encoding='utf-8') as f:
                                    f.write("\n" + "=" * 40 + "\n")
                                    f.write(f"PAGE {page_num}\n")
                                    f.write("=" * 40 + "\n\n")
                                    f.write(cached_content + "\n\n")
                            else:
                                self.logger.log(f"{Colors.YELLOW}Warning: Cache entry exists but content not found, "
                                      f"reprocessing page {page_num}{Colors.NC}")
                                should_process = True
                                skip_count -= 1

                    if should_process:
                        # Build context from previous pages
                        context_text = None
                        if context_pages > 0:
                            context_text = []
                            for i in range(max(start_page, page_num - context_pages), page_num):
                                if i in page_content:
                                    context_text.append((i, page_content[i]))
                            if not context_text:
                                context_text = None

                        # Process the image with context
                        result = self.process_image(image_file, prompt, page_num, context_text, max_retries)

                        if result:
                            # Append result to output file
                            with open(output_path, 'a', encoding='utf-8') as f:
                                f.write("\n" + "=" * 40 + "\n")
                                f.write(f"PAGE {page_num}\n")
                                f.write("=" * 40 + "\n\n")
                                f.write(result + "\n\n")

                            success_count += 1

                            # Cache the result
                            if use_cache:
                                cache_manager.mark_page_cached(pdf_path, page_num)
                                cache_manager.save_page_content(pdf_path, page_num, result)
                        else:
                            fail_count += 1

                        # Rate limiting delay
                        time.sleep(1)

                    # Store page content for future context (if we have result)
                    if result:
                        page_content[page_num] = result
            else:
                # Concurrent processing (no context needed)
                def process_page_wrapper(page_info):
                    """Wrapper function for concurrent processing"""
                    page_num, image_file = page_info

                    # Check cache first
                    if use_cache and not (redo_pages and page_num in redo_pages):
                        if cache_manager.is_page_cached(pdf_path, page_num):
                            self.logger.log(f"{Colors.BLUE}⊙ Loading page {page_num} from cache{Colors.NC}")
                            cached_content = cache_manager.load_page_content(pdf_path, page_num)
                            if cached_content:
                                return (page_num, cached_content, True)  # (page_num, content, from_cache)

                    # Process the page
                    result = self.process_image(image_file, prompt, page_num, None, max_retries)
                    return (page_num, result, False)

                # Process pages concurrently
                with ThreadPoolExecutor(max_workers=concurrent_workers) as executor:
                    # Submit all tasks
                    future_to_page = {executor.submit(process_page_wrapper, page_info): page_info
                                     for page_info in pages_to_process}

                    # Collect results as they complete and cache immediately
                    results = []
                    for future in as_completed(future_to_page):
                        page_num, content, from_cache = future.result()
                        results.append((page_num, content, from_cache))

                        # Cache the result immediately as it completes (not from_cache)
                        if content and not from_cache and use_cache:
                            cache_manager.mark_page_cached(pdf_path, page_num)
                            cache_manager.save_page_content(pdf_path, page_num, content)

                # Sort results by page number
                results.sort(key=lambda x: x[0])

                # Write results to file in order and count statistics
                for page_num, content, from_cache in results:
                    if content:
                        # Append result to output file
                        with open(output_path, 'a', encoding='utf-8') as f:
                            f.write("\n" + "=" * 40 + "\n")
                            f.write(f"PAGE {page_num}\n")
                            f.write("=" * 40 + "\n\n")
                            f.write(content + "\n\n")

                        if from_cache:
                            skip_count += 1
                        else:
                            success_count += 1
                    else:
                        fail_count += 1

            # Print summary
            self.logger.log("")
            self.logger.log(f"{Colors.GREEN}=== Processing Complete ==={Colors.NC}")
            self.logger.log(f"Total pages in PDF: {total_pages}")
            self.logger.log(f"Processed pages: {start_page} to {end_page}")
            self.logger.log(f"{Colors.GREEN}Newly processed: {success_count}{Colors.NC}")
            if skip_count > 0:
                self.logger.log(f"{Colors.BLUE}Loaded from cache: {skip_count}{Colors.NC}")
            if fail_count > 0:
                self.logger.log(f"{Colors.RED}Failed: {fail_count}{Colors.NC}")
            self.logger.log("")
            self.logger.log(f"Results saved to: {output_path}")
            if use_cache:
                cache_file = cache_manager.get_cache_file(pdf_path)
                self.logger.log(f"Cache file: {cache_file}")

        finally:
            # Cleanup temporary files
            if not keep_images:
                self.logger.log("\nCleaning up temporary files...")
                shutil.rmtree(temp_dir)
            else:
                self.logger.log(f"\nTemporary images kept in: {temp_dir}")


def main():
    parser = argparse.ArgumentParser(
        description='Convert PDF to images and process with Gemini API (OpenAI compatibility layer)',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s document.pdf results.txt --dpi 400
  %(prog)s document.pdf results.txt --redo-pages 1,5,10
  %(prog)s document.pdf results.txt --start-page 5 --num-pages 10
  %(prog)s document.pdf results.txt --context-pages 2
  %(prog)s document.pdf --clear-cache

Page Selection:
  --start-page N      Start processing from page N (default: 1)
  --num-pages N       Process N pages from start-page (default: all remaining)
  --context-pages N   Include N previous pages as context for each page (default: 0)

Cache:
  Processed pages are cached to avoid reprocessing.
  Cache is invalidated when the PDF file is modified.
  Use --redo-pages to reprocess specific pages.
  Use --no-cache to disable caching for this run.

Context Feature:
  When --context-pages is set, each page will receive the transcribed text
  from the specified number of previous pages as context, helping maintain
  continuity and improve accuracy for documents with flowing content.

Environment Variables:
  GEMINI_API_KEY  Your Gemini API Key (required)
        """
    )

    parser.add_argument('pdf_file', help='Path to the PDF file to process')
    parser.add_argument('output_file', nargs='?', default='output.txt',
                       help='Path to save the OCR results (default: output.txt)')
    parser.add_argument('--dpi', type=int, default=300,
                       help='DPI for image conversion (default: 300)')
    parser.add_argument('--prompt', help='Custom prompt for OCR')
    parser.add_argument('--prompt-file', help='Path to file containing custom prompt')
    parser.add_argument('--keep-images', action='store_true',
                       help='Keep temporary image files after processing')
    parser.add_argument('--redo-pages', help='Comma-separated list of page numbers to reprocess (e.g., 1,3,5)')
    parser.add_argument('--no-cache', action='store_true',
                       help='Disable caching and process all pages')
    parser.add_argument('--clear-cache', action='store_true',
                       help='Clear cache for this PDF and exit')
    parser.add_argument('--clear-all-cache', action='store_true',
                       help='Clear all cached data and exit')
    parser.add_argument('--context-pages', type=int, default=0,
                       help='Number of previous pages to include as context (default: 0)')
    parser.add_argument('--start-page', type=int, default=1,
                       help='Starting page number to process (default: 1)')
    parser.add_argument('--end-page', type=int,
                       help='Ending page number to process (inclusive)')
    parser.add_argument('--num-pages', type=int,
                       help='Number of pages to process (default: all remaining pages)')
    parser.add_argument('--max-retries', type=int, default=3,
                       help='Maximum number of retry attempts for failed pages (default: 3)')
    parser.add_argument('--concurrent-workers', type=int, default=5,
                       help='Number of concurrent workers for processing (default: 5, only used when context-pages=0)')
    parser.add_argument('--log-file', type=str, default='log.txt',
                       help='Path to log file (default: log.txt)')

    args = parser.parse_args()

    # Initialize logger
    logger = Logger(log_file=args.log_file if not (args.clear_cache or args.clear_all_cache) else None)

    # Check for API key
    api_key = os.environ.get('GEMINI_API_KEY')
    if not api_key and not (args.clear_cache or args.clear_all_cache):
        logger.log(f"{Colors.RED}Error: GEMINI_API_KEY environment variable is not set{Colors.NC}")
        logger.log("Please set your Gemini API Key:")
        logger.log("  export GEMINI_API_KEY='your-api-key-here'")
        logger.close()
        sys.exit(1)

    # Handle cache-only operations
    cache_manager = CacheManager()

    if args.clear_all_cache:
        cache_manager.clear_cache()
        sys.exit(0)

    if args.clear_cache:
        if not Path(args.pdf_file).exists():
            logger.log(f"{Colors.RED}Error: PDF file '{args.pdf_file}' not found{Colors.NC}")
            logger.close()
            sys.exit(1)
        cache_manager.clear_cache(args.pdf_file)
        logger.close()
        sys.exit(0)

    # Validate PDF file
    if not Path(args.pdf_file).exists():
        logger.log(f"{Colors.RED}Error: PDF file '{args.pdf_file}' not found{Colors.NC}")
        logger.close()
        sys.exit(1)

    # Parse redo pages
    redo_pages = None
    if args.redo_pages:
        try:
            redo_pages = [int(p.strip()) for p in args.redo_pages.split(',')]
        except ValueError:
            logger.log(f"{Colors.RED}Error: Invalid page numbers in --redo-pages{Colors.NC}")
            logger.close()
            sys.exit(1)

    # Validate start page
    if args.start_page < 1:
        logger.log(f"{Colors.RED}Error: --start-page must be >= 1{Colors.NC}")
        logger.close()
        sys.exit(1)

    # Validate num pages
    if args.num_pages is not None and args.num_pages < 1:
        logger.log(f"{Colors.RED}Error: --num-pages must be >= 1{Colors.NC}")
        logger.close()
        sys.exit(1)

    # Validate prompt arguments
    if args.prompt and args.prompt_file:
        logger.log(f"{Colors.RED}Error: Cannot specify both --prompt and --prompt-file{Colors.NC}")
        logger.close()
        sys.exit(1)

    # Load prompt from file
    if args.prompt_file:
        try:
            with open(args.prompt_file, 'r', encoding='utf-8') as f:
                args.prompt = f.read().strip()
        except Exception as e:
            logger.log(f"{Colors.RED}Error reading prompt file: {e}{Colors.NC}")
            logger.close()
            sys.exit(1)

    # Validate end page and conflicts
    if args.end_page is not None:
        if args.num_pages is not None:
            logger.log(f"{Colors.RED}Error: Cannot specify both --num-pages and --end-page{Colors.NC}")
            logger.close()
            sys.exit(1)
        if args.end_page < args.start_page:
            logger.log(f"{Colors.RED}Error: --end-page ({args.end_page}) cannot be less than --start-page ({args.start_page}){Colors.NC}")
            logger.close()
            sys.exit(1)
        args.num_pages = args.end_page - args.start_page + 1

    # Validate context pages
    if args.context_pages < 0:
        logger.log(f"{Colors.RED}Error: --context-pages must be >= 0{Colors.NC}")
        logger.close()
        sys.exit(1)

    # Validate max retries
    if args.max_retries < 1:
        logger.log(f"{Colors.RED}Error: --max-retries must be >= 1{Colors.NC}")
        logger.close()
        sys.exit(1)

    # Validate concurrent workers
    if args.concurrent_workers < 1:
        logger.log(f"{Colors.RED}Error: --concurrent-workers must be >= 1{Colors.NC}")
        logger.close()
        sys.exit(1)

    # Create processor and run
    processor = PDFOCRProcessor(api_key, logger)
    processor.check_dependencies()

    processor.process_pdf(
        args.pdf_file,
        args.output_file,
        dpi=args.dpi,
        prompt=args.prompt,
        use_cache=not args.no_cache,
        redo_pages=redo_pages,
        keep_images=args.keep_images,
        context_pages=args.context_pages,
        start_page=args.start_page,
        num_pages=args.num_pages,
        max_retries=args.max_retries,
        concurrent_workers=args.concurrent_workers
    )

    # Close logger
    logger.close()


if __name__ == '__main__':
    main()
