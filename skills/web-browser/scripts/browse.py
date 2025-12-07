#!/usr/bin/env python3
import os
import sys

# Auto-detect and use venv if not already active
script_dir = os.path.dirname(os.path.abspath(__file__))
venv_python = os.path.abspath(os.path.join(script_dir, "..", "venv", "bin", "python"))

if os.path.exists(venv_python) and sys.executable != venv_python:
    # Re-execute the script with the venv python
    os.execv(venv_python, [venv_python] + sys.argv)

import requests
from bs4 import BeautifulSoup

def browse_url(url, render_js=True):
    api_key = os.environ.get("SCRAPINGBEE_API_KEY")
    if not api_key:
        print("Error: SCRAPINGBEE_API_KEY environment variable not set.")
        sys.exit(1)

    sb_url = "https://app.scrapingbee.com/api/v1/"
    params = {
        "api_key": api_key,
        "url": url,
        "render_js": "true" if render_js else "false",
        "block_ads": "true", # Block ads to reduce noise
        "block_resources": "false" # Needed sometimes for layout, can set to true if pure text needed
    }

    print(f"Browsing {url} (JS: {render_js})...", file=sys.stderr)

    try:
        response = requests.get(sb_url, params=params)
        response.raise_for_status()
        
        soup = BeautifulSoup(response.content, 'html.parser')

        # Remove unwanted tags
        for script in soup(["script", "style", "nav", "footer", "header", "aside", "noscript", "iframe", "ad", "meta", "link"]):
            script.extract()

        # Get title
        title = soup.title.string if soup.title else "No Title"

        # Extract text
        # get_text with separator helps preserve structure better than just text
        text = soup.get_text(separator='\n\n')

        # Post-processing to clean up whitespace
        lines = (line.strip() for line in text.splitlines())
        chunks = (phrase.strip() for line in lines for phrase in line.split("  "))
        text = '\n'.join(chunk for chunk in chunks if chunk)

        # Output
        print(f"Title: {title}")
        print(f"URL: {url}")
        print("-" * 40)
        print(text)
        print("-" * 40)

    except requests.exceptions.RequestException as e:
        print(f"Error fetching URL via ScrapingBee: {e}")
        # If response exists, print it for debugging
        if 'response' in locals() and response is not None:
             print(f"Status Code: {response.status_code}")
             print(f"Response: {response.text[:500]}") # Print first 500 chars
        sys.exit(1)
    except Exception as e:
        print(f"Error processing content: {e}")
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: browse.py <url> [--no-js]")
        sys.exit(1)
    
    target_url = sys.argv[1]
    render_javascript = "--no-js" not in sys.argv
    
    browse_url(target_url, render_js=render_javascript)
