#!/usr/bin/env python3
import os
import sys

# Auto-detect and use venv if not already active
script_dir = os.path.dirname(os.path.abspath(__file__))
venv_python = os.path.abspath(os.path.join(script_dir, "..", "venv", "bin", "python"))

if os.path.exists(venv_python) and sys.executable != venv_python:
    os.execv(venv_python, [venv_python] + sys.argv)

import requests

def take_screenshot(url, output_path, full_page=True):
    api_key = os.environ.get("SCRAPINGBEE_API_KEY")
    if not api_key:
        print("Error: SCRAPINGBEE_API_KEY environment variable not set.")
        sys.exit(1)

    sb_url = "https://app.scrapingbee.com/api/v1/"
    params = {
        "api_key": api_key,
        "url": url,
        "screenshot": "true",
        "screenshot_full_page": "true" if full_page else "false",
        "block_ads": "true",
        "window_width": "1920",
        "window_height": "1080"
    }

    print(f"Taking screenshot of {url} (Full page: {full_page})...", file=sys.stderr)

    try:
        response = requests.get(sb_url, params=params, stream=True)
        response.raise_for_status()
        
        # Check if content type is an image
        content_type = response.headers.get('Content-Type', '')
        if 'image' not in content_type:
             print(f"Error: Expected image response, got {content_type}")
             # Try to print error message from body if it's text
             try:
                print(f"Response body: {response.text[:500]}")
             except:
                pass
             sys.exit(1)

        with open(output_path, 'wb') as f:
            for chunk in response.iter_content(chunk_size=8192):
                f.write(chunk)
        
        print(f"Screenshot saved to: {output_path}")

    except requests.exceptions.RequestException as e:
        print(f"Error taking screenshot via ScrapingBee: {e}")
        if 'response' in locals() and response is not None:
             print(f"Status Code: {response.status_code}")
             try:
                 print(f"Response: {response.text[:500]}")
             except:
                 pass
        sys.exit(1)
    except Exception as e:
        print(f"Error processing screenshot: {e}")
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) < 3:
        print("Usage: screenshot.py <url> <output_path> [--no-full-page]")
        sys.exit(1)
    
    target_url = sys.argv[1]
    output_file = sys.argv[2]
    full_page_flag = "--no-full-page" not in sys.argv
    
    take_screenshot(target_url, output_file, full_page=full_page_flag)
