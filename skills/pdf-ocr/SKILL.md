---
name: pdf-ocr
description: Convert PDF documents to text using AI-powered OCR (Gemini). Use when you need to extract text from scanned PDFs, images within PDFs, or when standard text extraction fails.
---

# Pdf Ocr

## Overview

This skill provides a robust, AI-powered OCR tool that converts PDF documents to text using Google's Gemini model (via OpenAI compatibility layer). It handles scanned documents, complex layouts, and handwriting better than traditional OCR tools.

Key features:
- **AI Accuracy**: Uses Gemini 3 Pro Preview for high-quality transcription.
- **Context-Aware**: Optional sequential mode uses previous pages to improve continuity.
- **Smart Caching**: Caches results to avoid redundant API calls.
- **Concurrency**: fast parallel processing for non-contextual tasks.

## Requirements

Before using this skill, ensure the following are set up:

1.  **System Dependencies**: `poppler-utils` (for `pdftoppm`)
    - Ubuntu/Debian: `sudo apt-get install poppler-utils`
    - macOS: `brew install poppler`
2.  **Python Packages**: `openai`
    - `pip install openai`
3.  **Environment Variable**: One of the following must be set:
    - `GEMINI_API_KEY`: Primary choice for this skill.
    - `API_KEY`: Standard variable used by the agent for Gemini.
    - `OPENAI_API_KEY`: Fallback if using Gemini through an OpenAI-compatible endpoint.
    - Example: `export API_KEY='your_api_key'`

## Usage

The main script is located at `skills/pdf-ocr/scripts/ocr.py`.

### Basic Extraction

```bash
skills/pdf-ocr/scripts/ocr.py input.pdf output.txt
```

### Advanced Options

- **Process specific pages**:
  ```bash
  skills/pdf-ocr/scripts/ocr.py input.pdf output.txt --start-page 1 --end-page 5
  ```

- **Use Context (Sequential Mode)**:
  Useful for books or articles where flow matters.
  ```bash
  skills/pdf-ocr/scripts/ocr.py input.pdf output.txt --context-pages 2
  ```

- **Redo specific pages**:
  ```bash
  skills/pdf-ocr/scripts/ocr.py input.pdf output.txt --redo-pages 5,12
  ```

- **Custom Prompt**:
  ```bash
  skills/pdf-ocr/scripts/ocr.py input.pdf output.txt --prompt "Summarize this page"
  ```
