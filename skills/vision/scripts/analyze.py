#!/usr/bin/env python3
import os
import sys
import argparse
import base64
import mimetypes
import io
import uuid
import requests
from PIL import Image
from openai import OpenAI

def encode_image(image_path):
    with open(image_path, "rb") as image_file:
        return base64.b64encode(image_file.read()).decode('utf-8')

def get_image_metadata(image_data):
    try:
        img = Image.open(io.BytesIO(image_data))
        return {
            "width": img.width,
            "height": img.height,
            "format": img.format,
            "size": len(image_data)
        }
    except Exception:
        return {"width": 0, "height": 0, "format": "UNKNOWN", "size": len(image_data)}

def main():
    parser = argparse.ArgumentParser(description="Analyze images using Gemini via OpenAI compatibility.")
    parser.add_argument("--image", required=True, help="Path to local image file or URL.")
    parser.add_argument("--output", help="Path to save the analysis output text.")
    parser.add_argument("--prompt", default="Describe this image in detail.", help="Prompt for the model.")
    args = parser.parse_args()

    api_key = os.environ.get("GEMINI_API_KEY")
    if not api_key:
        print("Error: GEMINI_API_KEY environment variable not set.", file=sys.stderr)
        sys.exit(1)

    client = OpenAI(
        api_key=api_key,
        base_url="https://generativelanguage.googleapis.com/v1beta/openai/"
    )

    image_input = args.image
    
    image_data = None
    mime_type = "image/jpeg"

    if image_input.lower().startswith("http://") or image_input.lower().startswith("https://"):
        try:
            response = requests.get(image_input)
            response.raise_for_status()
            mime_type = response.headers.get('Content-Type', mime_type)
            image_data = response.content
        except Exception as e:
            print(f"Error downloading image from URL: {e}", file=sys.stderr)
            sys.exit(1)
    else:
        # It's a local file
        if not os.path.exists(image_input):
            print(f"Error: File not found: {image_input}", file=sys.stderr)
            sys.exit(1)
            
        try:
            mime_type, _ = mimetypes.guess_type(image_input)
            if not mime_type:
                mime_type = "image/jpeg"

            with open(image_input, "rb") as f:
                image_data = f.read()
        except Exception as e:
            print(f"Error reading file: {e}", file=sys.stderr)
            sys.exit(1)

    # Prepare image object
    base64_image = base64.b64encode(image_data).decode('utf-8')
    image_url_obj = {"url": f"data:{mime_type};base64,{base64_image}"}

    # Get metadata
    metadata = get_image_metadata(image_data)

    try:
        response = client.chat.completions.create(
            model="gemini-2.5-flash",
            messages=[
                {
                    "role": "user",
                    "content": [
                        {"type": "text", "text": args.prompt},
                        {
                            "type": "image_url",
                            "image_url": image_url_obj,
                        },
                    ],
                }
            ],
        )
        content = response.choices[0].message.content
        
        # Determine output path
        output_path = args.output
        if not output_path:
            # Generate random filename
            random_suffix = uuid.uuid4().hex[:8]
            output_path = f"vision_analysis_{random_suffix}.txt"

        with open(output_path, "w") as f:
            f.write(content)

        print(f"Analysis saved to: {output_path}")
        print("Image Metadata:")
        print(f"  - Size: {metadata['size'] / 1024:.2f} KB")
        print(f"  - Dimensions: {metadata['width']}x{metadata['height']}")
        print(f"  - Format: {metadata['format']}")
    except Exception as e:
        print(f"Error calling API: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
