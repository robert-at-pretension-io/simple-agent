---
name: web-browser
description: Search the web and browse/scrape web pages to extract text content. Use this skill to find information online or read the content of specific URLs.
---

# Web Browser Skill

This skill provides capabilities to search the web using Brave Search and browse/scrape web pages using ScrapingBee. It is designed to extract readable text content from websites, filtering out boilerplate and code.

## Capabilities

1.  **Search**: Query the web for information.
2.  **Browse**: Visit a specific URL and extract its main text content.
3.  **Screenshot**: Capture a visual screenshot of a web page.

## Prerequisites

This skill requires the following environment variables to be set (typically in `~/.bashrc`):

- `BRAVE_SEARCH_API_KEY`: For search functionality.
- `SCRAPINGBEE_API_KEY`: For browsing/scraping functionality.

Dependencies:
- `requests`
- `beautifulsoup4`
- `lxml`

## Workflow Tips

- **Search first, then Browse**: Use the search script to discover relevant URLs, then use the browse script to read the content of the most promising results.
- **Output Format**: Both scripts output text to `stdout`. Search results are formatted as a Markdown list. Browse results are plain text with headers and footers.

## Scripts

### 1. Search

Search the web for a query. Returns a list of results with titles, URLs, and descriptions.

```bash
skills/web-browser/scripts/search.py "query string"
```

### 2. Browse

Visit a URL and extract text. Removes ads, scripts, and navigation to focus on content.

```bash
skills/web-browser/scripts/browse.py <url> [--js]
```

- `<url>`: The URL to visit.
- `--js`: (Optional) Enable JavaScript rendering (uses more credits/time, but helpful for dynamic sites).

### 3. Screenshot

Capture a screenshot of a webpage.

```bash
skills/web-browser/scripts/screenshot.py <url> <output_path> [--full-page]
```

## Examples

**Search for a topic:**
```bash
skills/web-browser/scripts/search.py "latest RedwoodJS features"
```

**Read a specific article:**
```bash
skills/web-browser/scripts/browse.py "https://redwoodjs.com/docs/introduction"
```

**Read a Single Page Application (SPA):**
```bash
skills/web-browser/scripts/browse.py "https://example.com/spa-app" --js
```

**Take a screenshot:**
```bash
skills/web-browser/scripts/screenshot.py "https://google.com" google.png
```
