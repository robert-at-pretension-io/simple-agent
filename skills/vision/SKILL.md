---
name: vision
description: Analyze images from local files or URLs using advanced computer vision models (Gemini).
---

# Vision Skill

Use this skill to look at images and answer questions about them.
It supports both local file paths and remote URLs.

## Usage

Use the 'run_script' tool to execute the analysis script.

### Analyze an Image
path: skills/vision/scripts/analyze.py
args:
  - "--image"
  - "/path/to/image.jpg" (or "https://example.com/image.jpg")
  - "--output"
  - "/path/to/output.txt" (Optional, defaults to random filename)
  - "--image-model"
  - "gemini-2.5-flash" (Optional, default: gemini-2.5-flash)
  - "--prompt"
  - "Describe what you see in this image." (Optional)

## Examples

1. **Analyze a local file**:
   skills/vision/scripts/analyze.py --image /home/elliot/diagram.png --output diagram.txt

2. **Analyze a URL with a specific question**:
   skills/vision/scripts/analyze.py --image https://example.com/chart.png --prompt "Extract the data from the bar chart."

## Notes
- Supports JPEG, PNG, WEBP, HEIC, HEIF.
- Uses Gemini 2.5 Flash model by default (configurable via --image-model) via OpenAI compatibility layer.
- Saves analysis to a text file and prints image metadata to stdout.
- Requires GEMINI_API_KEY environment variable.
