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
import json

def search_brave(query):
    api_key = os.environ.get("BRAVE_SEARCH_API_KEY")
    if not api_key:
        print("Error: BRAVE_SEARCH_API_KEY environment variable not set.")
        sys.exit(1)

    url = "https://api.search.brave.com/res/v1/web/search"
    headers = {
        "X-Subscription-Token": api_key,
        "Accept": "application/json"
    }
    params = {
        "q": query
    }

    try:
        response = requests.get(url, headers=headers, params=params)
        response.raise_for_status()
        data = response.json()
        
        results = data.get("web", {}).get("results", [])
        if not results:
            print("No results found.")
            return

        print(f"# Search Results for: {query}\n")
        for i, result in enumerate(results, 1):
            title = result.get("title", "No Title")
            link = result.get("url", "#")
            description = result.get("description", "No description available.")
            print(f"{i}. [{title}]({link})")
            print(f"   {description}\n")

    except requests.exceptions.RequestException as e:
        print(f"Error querying Brave Search API: {e}")
        sys.exit(1)
    except ValueError as e: # includes JSONDecodeError
        print(f"Error parsing API response: {e}")
        sys.exit(1)

if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: search.py <query>")
        sys.exit(1)
    
    query = " ".join(sys.argv[1:])
    search_brave(query)
