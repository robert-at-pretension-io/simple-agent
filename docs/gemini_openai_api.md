Skip to main content
Gemini API
Search
/


English
Get API key
Cookbook
Community
Sign in
Gemini API Docs
API reference

Gemini 3 is here. Read the developer guide to get started with our most advanced model yet.
Home
Gemini API
Gemini API Docs
Was this helpful?

Send feedbackThought Signatures

content_copy




Thought signatures are encrypted representations of the model's internal thought process and are used to preserve reasoning context across multi-turn interactions. When using thinking models (such as the Gemini 3 and 2.5 series), the API may return a thoughtSignature field within the content parts of the response (e.g., text or functionCall parts).

As a general rule, if you receive a thought signature in a model response, you should pass it back exactly as received when sending the conversation history in the next turn. When using Gemini 3 Pro, you must pass back thought signatures during function calling, otherwise you will get a validation error (4xx status code).

Note: If you use the official Google Gen AI SDKs and use the chat feature (or append the full model response object directly to history), thought signatures are handled automatically. You do not need to manually extract or manage them, or change your code.
How it works
The graphic below visualizes the meaning of "turn" and "step" as they pertain to function calling in the Gemini API. A "turn" is a single, complete exchange in a conversation between a user and a model. A "step" is a finer-grained action or operation performed by the model, often as part of a larger process to complete a turn.

Function calling turns and steps diagram

This document focuses on handling function calling for Gemini 3 Pro. Refer to the model behavior section for discrepancies with 2.5.

Gemini 3 Pro returns thought signatures for all model responses (responses from the API) with a function call. Thought signatures show up in the following cases:

When there are parallel function calls, the first function call part returned by the model response will have a thought signature.
When there are sequential function calls (multi-step), each function call will have a signature and you must pass all signatures back.
Model responses without a function call will return a thought signature inside the last part returned by the model.
The following table provides a visualization for multi-step function calls, combining the definitions of turns and steps with the concept of signatures introduced above:

Turn

Step

User Request

Model Response

FunctionResponse

1

1

request1 = user_prompt	FC1 + signature	FR1
1

2

request2 = request1 + (FC1 + signature) + FR1	FC2 + signature	FR2
1

3

request3 = request2 + (FC2 + signature) + FR2	text_output
(no FCs)

None

Signatures in function calling parts
When Gemini generates a functionCall, it relies on the thought_signature to process the tool's output correctly in the next turn.

Behavior:
Single Function Call: The functionCall part will contain a thought_signature.
Parallel Function Calls: If the model generates parallel function calls in a response, the thought_signature is attached only to the first functionCall part. Subsequent functionCall parts in the same response will not contain a signature.
Requirement: You must return this signature in the exact part where it was received when sending the conversation history back.
Validation: Strict validation is enforced for all function calls within the current turn . (Only current turn is required; we don't validate on previous turns)
The API goes back in the history (newest to oldest) to find the most recent User message that contains standard content (e.g., text) ( which would be the start of the current turn). This will not be a functionResponse.
All model functionCall turns occurring after that specific use message are considered part of the turn.
The first functionCall part in each step of the current turn must include its thought_signature.
If you omit a thought_signature for the first functionCall part in any step of the current turn, the request will fail with a 400 error.
If proper signatures are not returned, here is how you will error out
gemini-3-pro-preview: Failure to include signatures will result in a 400 error. The verbiage will be of the form :
Function call <Function Call> in the <index of contents array> content block is missing a thought_signature. For example, Function call FC1 in the 1. content block is missing a thought_signature.
Sequential function calling example
This section shows an example of multiple function calls where the user asks a complex question requiring multiple tasks.

Let's walk through a multiple-turn function calling example where the user asks a complex question requiring multiple tasks: "Check flight status for AA100 and book a taxi if delayed".

Turn

Step

User Request

Model Response

FunctionResponse

1

1

request1="Check flight status for AA100 and book a taxi 2 hours before if delayed."	FC1 ("check_flight") + signature	FR1
1

2

request2 = request1 + FC1 ("check_flight") + signature + FR1	FC2("book_taxi") + signature	FR2
1

3

request3 = request2 + FC2 ("book_taxi") + signature + FR2	text_output
(no FCs)

None
The following code illustrates the sequence in the above table.

Turn 1, Step 1 (User request)


{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "Check flight status for AA100 and book a taxi 2 hours before if delayed."
        }
      ]
    }
  ],
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "check_flight",
          "description": "Gets the current status of a flight",
          "parameters": {
            "type": "object",
            "properties": {
              "flight": {
                "type": "string",
                "description": "The flight number to check"
              }
            },
            "required": [
              "flight"
            ]
          }
        },
        {
          "name": "book_taxi",
          "description": "Book a taxi",
          "parameters": {
            "type": "object",
            "properties": {
              "time": {
                "type": "string",
                "description": "time to book the taxi"
              }
            },
            "required": [
              "time"
            ]
          }
        }
      ]
    }
  ]
}
Turn 1, Step 1 (Model response)


{
"content": {
        "role": "model",
        "parts": [
          {
            "functionCall": {
              "name": "check_flight",
              "args": {
                "flight": "AA100"
              }
            },
            "thoughtSignature": "<Signature A>"
          }
        ]
  }
}
Turn 1, Step 2 (User response - Sending tool outputs) Since this user turn only contains a functionResponse (no fresh text), we are still in Turn 1. We must preserve <Signature_A>.


{
      "role": "user",
      "parts": [
        {
          "text": "Check flight status for AA100 and book a taxi 2 hours before if delayed."
        }
      ]
    },
    {
        "role": "model",
        "parts": [
          {
            "functionCall": {
              "name": "check_flight",
              "args": {
                "flight": "AA100"
              }
            },
            "thoughtSignature": "<Signature A>" //Required and Validated
          }
        ]
      },
      {
        "role": "user",
        "parts": [
          {
            "functionResponse": {
              "name": "check_flight",
              "response": {
                "status": "delayed",
                "departure_time": "12 PM"
                }
              }
            }
        ]
}
Turn 1, Step 2 (Model) The model now decides to book a taxi based on the previous tool output.

{
      "content": {
        "role": "model",
        "parts": [
          {
            "functionCall": {
              "name": "book_taxi",
              "args": {
                "time": "10 AM"
              }
            },
            "thoughtSignature": "<Signature B>"
          }
        ]
      }
}
Turn 1, Step 3 (User - Sending tool output) To send the taxi booking confirmation, we must include signatures for ALL function calls in this loop (<Signature A> + <Signature B>).

{
      "role": "user",
      "parts": [
        {
          "text": "Check flight status for AA100 and book a taxi 2 hours before if delayed."
        }
      ]
    },
    {
        "role": "model",
        "parts": [
          {
            "functionCall": {
              "name": "check_flight",
              "args": {
                "flight": "AA100"
              }
            },
            "thoughtSignature": "<Signature A>" //Required and Validated
          }
        ]
      },
      {
        "role": "user",
        "parts": [
          {
            "functionResponse": {
              "name": "check_flight",
              "response": {
                "status": "delayed",
                "departure_time": "12 PM"
              }
              }
            }
        ]
      },
      {
        "role": "model",
        "parts": [
          {
            "functionCall": {
              "name": "book_taxi",
              "args": {
                "time": "10 AM"
              }
            },
            "thoughtSignature": "<Signature B>" //Required and Validated
          }
        ]
      },
      {
        "role": "user",
        "parts": [
          {
            "functionResponse": {
              "name": "book_taxi",
              "response": {
                "booking_status": "success"
              }
              }
            }
        ]
    }
}
Parallel function calling example
Let's walk through a parallel function calling example where the users asks "Check weather in Paris and London" to see where the model does validation.

Turn

Step

User Request

Model Response

FunctionResponse

1

1

request1="Check the weather in Paris and London"

FC1 ("Paris") + signature

FC2 ("London")

FR1

1

2

request 2 = request1 + FC1 ("Paris") + signature + FC2 ("London")

text_output

(no FCs)

None

The following code illustrates the sequence in the above table.

Turn 1, Step 1 (User request)

{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "Check the weather in Paris and London."
        }
      ]
    }
  ],
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "get_current_temperature",
          "description": "Gets the current temperature for a given location.",
          "parameters": {
            "type": "object",
            "properties": {
              "location": {
                "type": "string",
                "description": "The city name, e.g. San Francisco"
              }
            },
            "required": [
              "location"
            ]
          }
        }
      ]
    }
  ]
}
Turn 1, Step 1 (Model response)

{
  "content": {
    "parts": [
      {
        "functionCall": {
          "name": "get_current_temperature",
          "args": {
            "location": "Paris"
          }
        },
        "thoughtSignature": "<Signature_A>"// INCLUDED on First FC
      },
      {
        "functionCall": {
          "name": "get_current_temperature",
          "args": {
            "location": "London"
          }// NO signature on subsequent parallel FCs
        }
      }
    ]
  }
}
Turn 1, Step 2 (User response - Sending tool outputs) We must preserve <Signature_A> on the first part exactly as received.

[
  {
    "role": "user",
    "parts": [
      {
        "text": "Check the weather in Paris and London."
      }
    ]
  },
  {
    "role": "model",
    "parts": [
      {
        "functionCall": {
          "name": "get_current_temperature",
          "args": {
            "city": "Paris"
          }
        },
        "thought_signature": "<Signature_A>" // MUST BE INCLUDED
      },
      {
        "functionCall": {
          "name": "get_current_temperature",
          "args": {
            "city": "London"
          }
        }
      } // NO SIGNATURE FIELD
    ]
  },
  {
    "role": "user",
    "parts": [
      {
        "functionResponse": {
          "name": "get_current_temperature",
          "response": {
            "temp": "15C"
          }
        }
      },
      {
        "functionResponse": {
          "name": "get_current_temperature",
          "response": {
            "temp": "12C"
          }
        }
      }
    ]
  }
]
Signatures in non functionCall parts
Gemini may also return thought_signatures in the final part of the response in non-function-call parts.

Behavior: The final content part (text, inlineData…) returned by the model may contain a thought_signature.
Recommendation: Returning these signatures is recommended to ensure the model maintains high-quality reasoning, especially for complex instruction following or simulated agentic workflows.
Validation: The API does not strictly enforce validation. You won't receive a blocking error if you omit them, though performance may degrade.
Text/In-context reasoning (No validation)
Turn 1, Step 1 (Model response)

{
  "role": "model",
  "parts": [
    {
      "text": "I need to calculate the risk. Let me think step-by-step...",
      "thought_signature": "<Signature_C>" // OPTIONAL (Recommended)
    }
  ]
}
Turn 2, Step 1 (User)

[
  { "role": "user", "parts": [{ "text": "What is the risk?" }] },
  {
    "role": "model", 
    "parts": [
      {
        "text": "I need to calculate the risk. Let me think step-by-step...",
        // If you omit <Signature_C> here, no error will occur.
      }
    ]
  },
  { "role": "user", "parts": [{ "text": "Summarize it." }] }
]
Signatures for OpenAI compatibility
The following examples shows how to handle thought signatures for a chat completion API using OpenAI compatibility.

Sequential function calling example
This is an example of multiple function calling where the user asks a complex question requiring multiple tasks.

Let's walk through a multiple-turn function calling example where the user asks Check flight status for AA100 and book a taxi if delayed and you can see what happens when the user asks a complex question requiring multiple tasks.

Turn

Step

User Request

Model Response

FunctionResponse

1

1

request1="Check the weather in Paris and London"	FC1 ("Paris") + signature
FC2 ("London")

FR1
1

2

request 2 = request1 + FC1 ("Paris") + signature + FC2 ("London")	text_output
(no FCs)

None
The following code walks through the given sequence.

Turn 1, Step 1 (User Request)

{
  "model": "google/gemini-3-pro-preview",
  "messages": [
    {
      "role": "user",
      "content": "Check flight status for AA100 and book a taxi 2 hours before if delayed."
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "check_flight",
        "description": "Gets the current status of a flight",
        "parameters": {
          "type": "object",
          "properties": {
            "flight": {
              "type": "string",
              "description": "The flight number to check."
            }
          },
          "required": [
            "flight"
          ]
        }
      }
    },
    {
      "type": "function",
      "function": {
        "name": "book_taxi",
        "description": "Book a taxi",
        "parameters": {
          "type": "object",
          "properties": {
            "time": {
              "type": "string",
              "description": "time to book the taxi"
            }
          },
          "required": [
            "time"
          ]
        }
      }
    }
  ]
}
Turn 1, Step 1 (Model Response)

{
      "role": "model",
        "tool_calls": [
          {
            "extra_content": {
              "google": {
                "thought_signature": "<Signature A>"
              }
            },
            "function": {
              "arguments": "{\"flight\":\"AA100\"}",
              "name": "check_flight"
            },
            "id": "function-call-1",
            "type": "function"
          }
        ]
    }
Turn 1, Step 2 (User Response - Sending Tool Outputs)

Since this user turn only contains a functionResponse (no fresh text), we are still in Turn 1 and must preserve <Signature_A>.

"messages": [
    {
      "role": "user",
      "content": "Check flight status for AA100 and book a taxi 2 hours before if delayed."
    },
    {
      "role": "model",
        "tool_calls": [
          {
            "extra_content": {
              "google": {
                "thought_signature": "<Signature A>" //Required and Validated
              }
            },
            "function": {
              "arguments": "{\"flight\":\"AA100\"}",
              "name": "check_flight"
            },
            "id": "function-call-1",
            "type": "function"
          }
        ]
    },
    {
      "role": "tool",
      "name": "check_flight",
      "tool_call_id": "function-call-1",
      "content": "{\"status\":\"delayed\",\"departure_time\":\"12 PM\"}"                 
    }
  ]
Turn 1, Step 2 (Model)

The model now decides to book a taxi based on the previous tool output.

{
"role": "model",
"tool_calls": [
{
"extra_content": {
"google": {
"thought_signature": "<Signature B>"
}
            },
            "function": {
              "arguments": "{\"time\":\"10 AM\"}",
              "name": "book_taxi"
            },
            "id": "function-call-2",
            "type": "function"
          }
       ]
}
Turn 1, Step 3 (User - Sending Tool Output)

To send the taxi booking confirmation, we must include signatures for ALL function calls in this loop (<Signature A> + <Signature B>).

"messages": [
    {
      "role": "user",
      "content": "Check flight status for AA100 and book a taxi 2 hours before if delayed."
    },
    {
      "role": "model",
        "tool_calls": [
          {
            "extra_content": {
              "google": {
                "thought_signature": "<Signature A>" //Required and Validated
              }
            },
            "function": {
              "arguments": "{\"flight\":\"AA100\"}",
              "name": "check_flight"
            },
            "id": "function-call-1d6a1a61-6f4f-4029-80ce-61586bd86da5",
            "type": "function"
          }
        ]
    },
    {
      "role": "tool",
      "name": "check_flight",
      "tool_call_id": "function-call-1d6a1a61-6f4f-4029-80ce-61586bd86da5",
      "content": "{\"status\":\"delayed\",\"departure_time\":\"12 PM\"}"                 
    },
    {
      "role": "model",
        "tool_calls": [
          {
            "extra_content": {
              "google": {
                "thought_signature": "<Signature B>" //Required and Validated
              }
            },
            "function": {
              "arguments": "{\"time\":\"10 AM\"}",
              "name": "book_taxi"
            },
            "id": "function-call-65b325ba-9b40-4003-9535-8c7137b35634",
            "type": "function"
          }
        ]
    },
    {
      "role": "tool",
      "name": "book_taxi",
      "tool_call_id": "function-call-65b325ba-9b40-4003-9535-8c7137b35634",
      "content": "{\"booking_status\":\"success\"}"
    }
  ]
Parallel function calling example
Let's walk through a parallel function calling example where the users asks "Check weather in Paris and London" and you can see where the model does validation.

Turn

Step

User Request

Model Response

FunctionResponse

1

1

request1="Check the weather in Paris and London"	FC1 ("Paris") + signature
FC2 ("London")

FR1
1

2

request 2 = request1 + FC1 ("Paris") + signature + FC2 ("London")	text_output
(no FCs)

None
Here's the code to walk through the given sequence.

Turn 1, Step 1 (User Request)

{
  "contents": [
    {
      "role": "user",
      "parts": [
        {
          "text": "Check the weather in Paris and London."
        }
      ]
    }
  ],
  "tools": [
    {
      "functionDeclarations": [
        {
          "name": "get_current_temperature",
          "description": "Gets the current temperature for a given location.",
          "parameters": {
            "type": "object",
            "properties": {
              "location": {
                "type": "string",
                "description": "The city name, e.g. San Francisco"
              }
            },
            "required": [
              "location"
            ]
          }
        }
      ]
    }
  ]
}
Turn 1, Step 1 (Model Response)

{
"role": "assistant",
        "tool_calls": [
          {
            "extra_content": {
              "google": {
                "thought_signature": "<Signature A>" //Signature returned
              }
            },
            "function": {
              "arguments": "{\"location\":\"Paris\"}",
              "name": "get_current_temperature"
            },
            "id": "function-call-f3b9ecb3-d55f-4076-98c8-b13e9d1c0e01",
            "type": "function"
          },
          {
            "function": {
              "arguments": "{\"location\":\"London\"}",
              "name": "get_current_temperature"
            },
            "id": "function-call-335673ad-913e-42d1-bbf5-387c8ab80f44",
            "type": "function" // No signature on Parallel FC
          }
        ]
}
Turn 1, Step 2 (User Response - Sending Tool Outputs)

You must preserve <Signature_A> on the first part exactly as received.

"messages": [
    {
      "role": "user",
      "content": "Check the weather in Paris and London."
    },
    {
      "role": "assistant",
        "tool_calls": [
          {
            "extra_content": {
              "google": {
                "thought_signature": "<Signature A>" //Required
              }
            },
            "function": {
              "arguments": "{\"location\":\"Paris\"}",
              "name": "get_current_temperature"
            },
            "id": "function-call-f3b9ecb3-d55f-4076-98c8-b13e9d1c0e01",
            "type": "function"
          },
          {
            "function": { //No Signature
              "arguments": "{\"location\":\"London\"}",
              "name": "get_current_temperature"
            },
            "id": "function-call-335673ad-913e-42d1-bbf5-387c8ab80f44",
            "type": "function"
          }
        ]
    },
    {
      "role":"tool",
      "name": "get_current_temperature",
      "tool_call_id": "function-call-f3b9ecb3-d55f-4076-98c8-b13e9d1c0e01",
      "content": "{\"temp\":\"15C\"}"
    },    
    {
      "role":"tool",
      "name": "get_current_temperature",
      "tool_call_id": "function-call-335673ad-913e-42d1-bbf5-387c8ab80f44",
      "content": "{\"temp\":\"12C\"}"
    }
  ]
FAQs
How do I transfer history from a different model to Gemini 3 Pro with a function call part in the current turn and step? I need to provide function call parts that were not generated by the API and therefore don't have an associated thought signature?

While injecting custom function call blocks into the request is strongly discouraged, in cases where it can't be avoided, e.g. providing information to the model on function calls and responses that were executed deterministically by the client, or transferring a trace from a different model that does not include thought signatures, you can set the following dummy signatures of either "context_engineering_is_the_way_to_go" or "skip_thought_signature_validator" in the thought signature field to skip validation.

I am sending back interleaved parallel function calls and responses and the API is returning a 400. Why?

When the API returns parallel function calls "FC1 + signature, FC2", the user response expected is "FC1+ signature, FC2, FR1, FR2". If you have them interleaved as "FC1 + signature, FR1, FC2, FR2" the API will return a 400 error.

When streaming and the model is not returning a function call I can't find the thought signature

During a model response not containing a FC with a streaming request, the model may return the thought signature in a part with an empty text content part. It is advisable to parse the entire request until the finish_reason is returned by the model.

Thought signature behavior by model series
Gemini 3 Pro and Gemini 2.5 models behave differently with thought signatures in function calls:

If there are function calls in a response,
Gemini 3 Pro will always have the signature on the first function call part. It is mandatory to return that part.
Gemini 2.5 will have the signature in the first part (regardless of type). It is optional to return that part.
If there are no function calls in a response,
Gemini 3 Pro will have the signature on the last part if the model generates a thought.
Gemini 2.5 won't have a signature in any part.
For Gemini 2.5 models thought signature behavior, refer to the Thinking page.

Was this helpful?

Send feedback
Except as otherwise noted, the content of this page is licensed under the Creative Commons Attribution 4.0 License, and code samples are licensed under the Apache 2.0 License. For details, see the Google Developers Site Policies. Java is a registered trademark of Oracle and/or its affiliates.

Last updated 2025-11-18 UTC.

Terms
Privacy

English


Skip to main content
Gemini API
Search
/


English
Get API key
Cookbook
Community
Sign in
Gemini API Docs
API reference

Gemini 3 is here. Read the developer guide to get started with our most advanced model yet.
Home
Gemini API
Gemini API Docs
Was this helpful?

Send feedbackGemini 3 Developer Guide

content_copy




Gemini 3 is our most intelligent model family to date, built on a foundation of state-of-the-art reasoning. It is designed to bring any idea to life by mastering agentic workflows, autonomous coding, and complex multimodal tasks. This guide covers key features of the Gemini 3 model family and how to get the most out of it.

High/Dynamic Thinking Low Thinking

Gemini 3 Pro uses dynamic thinking by default to reason through prompts. For faster, lower-latency responses when complex reasoning isn't required, you can constrain the model's thinking level to low.

Python
JavaScript
REST
curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-pro-preview:generateContent" \
  -H "x-goog-api-key: $GEMINI_API_KEY" \
  -H 'Content-Type: application/json' \
  -X POST \
  -d '{
    "contents": [{
      "parts": [{"text": "Find the race condition in this multi-threaded C++ snippet: [code here]"}]
    }]
  }'
Explore
Gemini 3 Applets Overview

Explore our collection of Gemini 3 apps to see how the model handles advanced reasoning, autonomous coding, and complex multimodal tasks.

Meet Gemini 3
Gemini 3 Pro is the first model in the new series. gemini-3-pro-preview is best for your complex tasks that require broad world knowledge and advanced reasoning across modalities.

Model ID	Context Window (In / Out)	Knowledge Cutoff	Pricing (Input / Output)*
gemini-3-pro-preview	1M / 64k	Jan 2025	$2 / $12 (<200k tokens)
$4 / $18 (>200k tokens)
gemini-3-pro-image-preview	65k / 32k	Jan 2025	$2 (Text Input) / $0.134 (Image Output)**
* Pricing is per 1 million tokens unless otherwise noted. ** Image pricing varies by resolution. See the pricing page for details.

For detailed rate limits, batch pricing, and additional information, see the models page.

New API features in Gemini 3
Gemini 3 introduces new parameters designed to give developers more control over latency, cost, and multimodal fidelity.

Thinking level
The thinking_level parameter controls the maximum depth of the model's internal reasoning process before it produces a response. Gemini 3 treats these levels as relative allowances for thinking rather than strict token guarantees. If thinking_level is not specified, Gemini 3 Pro will default to high.

low: Minimizes latency and cost. Best for simple instruction following, chat, or high-throughput applications
medium: (Coming soon), not supported at launch
high (Default): Maximizes reasoning depth. The model may take significantly longer to reach a first token, but the output will be more carefully reasoned.
Warning: You cannot use both thinking_level and the legacy thinking_budget parameter in the same request. Doing so will return a 400 error.
Media resolution
Gemini 3 introduces granular control over multimodal vision processing via the media_resolution parameter. Higher resolutions improve the model's ability to read fine text or identify small details, but increase token usage and latency. The media_resolution parameter determines the maximum number of tokens allocated per input image or video frame.

You can now set the resolution to media_resolution_low, media_resolution_medium, or media_resolution_high per individual media part or globally (via generation_config). If unspecified, the model uses optimal defaults based on the media type.

Recommended settings

Media Type	Recommended Setting	Max Tokens	Usage Guidance
Images	media_resolution_high	1120	Recommended for most image analysis tasks to ensure maximum quality.
PDFs	media_resolution_medium	560	Optimal for document understanding; quality typically saturates at medium. Increasing to high rarely improves OCR results for standard documents.
Video (General)	media_resolution_low (or media_resolution_medium)	70 (per frame)	Note: For video, low and medium settings are treated identically (70 tokens) to optimize context usage. This is sufficient for most action recognition and description tasks.
Video (Text-heavy)	media_resolution_high	280 (per frame)	Required only when the use case involves reading dense text (OCR) or small details within video frames.
Note: The media_resolution parameter maps to different token counts depending on the input type. While images scale linearly (media_resolution_low: 280, media_resolution_medium: 560, media_resolution_high: 1120), Video is compressed more aggressively. For Video, both media_resolution_low and media_resolution_medium are capped at 70 tokens per frame, and media_resolution_high is capped at 280 tokens. See full details here
Python
JavaScript
REST
curl "https://generativelanguage.googleapis.com/v1alpha/models/gemini-3-pro-preview:generateContent" \
  -H "x-goog-api-key: $GEMINI_API_KEY" \
  -H 'Content-Type: application/json' \
  -X POST \
  -d '{
    "contents": [{
      "parts": [
        { "text": "What is in this image?" },
        {
          "inlineData": {
            "mimeType": "image/jpeg",
            "data": "..."
          },
          "mediaResolution": {
            "level": "media_resolution_high"
          }
        }
      ]
    }]
  }'
Temperature
For Gemini 3, we strongly recommend keeping the temperature parameter at its default value of 1.0.

While previous models often benefited from tuning temperature to control creativity versus determinism, Gemini 3's reasoning capabilities are optimized for the default setting. Changing the temperature (setting it below 1.0) may lead to unexpected behavior, such as looping or degraded performance, particularly in complex mathematical or reasoning tasks.

Thought signatures
Gemini 3 uses Thought signatures to maintain reasoning context across API calls. These signatures are encrypted representations of the model's internal thought process. To ensure the model maintains its reasoning capabilities you must return these signatures back to the model in your request exactly as they were received:

Function Calling (Strict): The API enforces strict validation on the "Current Turn". Missing signatures will result in a 400 error.
Text/Chat: Validation is not strictly enforced, but omitting signatures will degrade the model's reasoning and answer quality.
Image generation/editing (Strict): The API enforces strict validation on all Model parts including a thoughtSignature. Missing signatures will result in a 400 error.
Success: If you use the official SDKs (Python, Node, Java) and standard chat history, Thought Signatures are handled automatically. You do not need to manually manage these fields.
Function calling (strict validation)
When Gemini generates a functionCall, it relies on the thoughtSignature to process the tool's output correctly in the next turn. The "Current Turn" includes all Model (functionCall) and User (functionResponse) steps that occurred since the last standard User text message.

Single Function Call: The functionCall part contains a signature. You must return it.
Parallel Function Calls: Only the first functionCall part in the list will contain the signature. You must return the parts in the exact order received.
Multi-Step (Sequential): If the model calls a tool, receives a result, and calls another tool (within the same turn), both function calls have signatures. You must return all accumulated signatures in the history.
Text and streaming
For standard chat or text generation, the presence of a signature is not guaranteed.

Non-Streaming: The final content part of the response may contain a thoughtSignature, though it is not always present. If one is returned, you should send it back to maintain best performance.
Streaming: If a signature is generated, it may arrive in a final chunk that contains an empty text part. Ensure your stream parser checks for signatures even if the text field is empty.
Image generation and editing
For gemini-3-pro-image-preview, thought signatures are critical for conversational editing. When you ask the model to modify an image it relies on the thoughtSignature from the previous turn to understand the composition and logic of the original image.

Editing: Signatures are guaranteed on the first part after the thoughts of the response (text or inlineData) and on every subsequent inlineData part. You must return all of these signatures to avoid errors.
Code examples
Multi-step Function Calling (Sequential)
Parallel Function Calling
Text/In-Context Reasoning (No Validation)
Image Generation & Editing
Migrating from other models
If you are transferring a conversation trace from another model (e.g., Gemini 2.5) or injecting a custom function call that was not generated by Gemini 3, you will not have a valid signature.

To bypass strict validation in these specific scenarios, populate the field with this specific dummy string: "thoughtSignature": "context_engineering_is_the_way_to_go"

Structured Outputs with tools
Gemini 3 allows you to combine Structured Outputs with built-in tools, including Grounding with Google Search, URL Context, and Code Execution.

Python
JavaScript
REST

curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-pro-preview:generateContent" \
  -H "x-goog-api-key: $GEMINI_API_KEY" \
  -H 'Content-Type: application/json' \
  -X POST \
  -d '{
    "contents": [{
      "parts": [{"text": "Search for all details for the latest Euro."}]
    }],
    "tools": [
      {"googleSearch": {}},
      {"urlContext": {}}
    ],
    "generationConfig": {
        "responseMimeType": "application/json",
        "responseJsonSchema": {
            "type": "object",
            "properties": {
                "winner": {"type": "string", "description": "The name of the winner."},
                "final_match_score": {"type": "string", "description": "The final score."},
                "scorers": {
                    "type": "array",
                    "items": {"type": "string"},
                    "description": "The name of the scorer."
                }
            },
            "required": ["winner", "final_match_score", "scorers"]
        }
    }
  }'
Image generation
Gemini 3 Pro Image lets you generate and edit images from text prompts. It uses reasoning to "think" through a prompt and can retrieve real-time data—such as weather forecasts or stock charts—before using Google Search grounding before generating high-fidelity images.

New & improved capabilities:

Native 4K & text rendering: Generate sharp, legible text and diagrams with native upscaling to 2K and 4K resolutions.
Grounded generation: Use the google_search tool to verify facts and generate imagery based on real-world information.
Conversational editing: Multi-turn image editing by simply asking for changes (e.g., "Make the background a sunset"). This workflow relies on Thought Signatures to preserve visual context between turns.
For complete details on aspect ratios, editing workflows, and configuration options, see the Image Generation guide.

Python
JavaScript
REST

curl "https://generativelanguage.googleapis.com/v1beta/models/gemini-3-pro-image-preview:generateContent" \
  -H "x-goog-api-key: $GEMINI_API_KEY" \
  -H 'Content-Type: application/json' \
  -X POST \
  -d '{
    "contents": [{
      "parts": [{"text": "Generate a visualization of the current weather in Tokyo."}]
    }],
    "tools": [{"googleSearch": {}}],
    "generationConfig": {
        "imageConfig": {
          "aspectRatio": "16:9",
          "imageSize": "4K"
      }
    }
  }'
Example Response

Weather Tokyo

Migrating from Gemini 2.5
Gemini 3 is our most capable model family to date and offers a stepwise improvement over Gemini 2.5 Pro. When migrating, consider the following:

Thinking: If you were previously using complex prompt engineering (like Chain-of-thought) to force Gemini 2.5 to reason, try Gemini 3 with thinking_level: "high" and simplified prompts.
Temperature settings: If your existing code explicitly sets temperature (especially to low values for deterministic outputs), we recommend removing this parameter and using the Gemini 3 default of 1.0 to avoid potential looping issues or performance degradation on complex tasks.
PDF & document understanding: Default OCR resolution for PDFs has changed. If you relied on specific behavior for dense document parsing, test the new media_resolution_high setting to ensure continued accuracy.
Token consumption: Migrating to Gemini 3 Pro defaults may increase token usage for PDFs but decrease token usage for video. If requests now exceed the context window due to higher default resolutions, we recommend explicitly reducing the media resolution.
Image segmentation: Image segmentation capabilities (returning pixel-level masks for objects) are not supported in Gemini 3 Pro. For workloads requiring native image segmentation, we recommend continuing to utilize Gemini 2.5 Flash with thinking turned off or Gemini Robotics-ER 1.5.
OpenAI compatibility
For users utilizing the OpenAI compatibility layer, standard parameters are automatically mapped to Gemini equivalents:

reasoning_effort (OAI) maps to thinking_level (Gemini). Note that reasoning_effort medium maps to thinking_level high.
Prompting best practices
Gemini 3 is a reasoning model, which changes how you should prompt.

Precise instructions: Be concise in your input prompts. Gemini 3 responds best to direct, clear instructions. It may over-analyze verbose or overly complex prompt engineering techniques used for older models.
Output verbosity: By default, Gemini 3 is less verbose and prefers providing direct, efficient answers. If your use case requires a more conversational or "chatty" persona, you must explicitly steer the model in the prompt (e.g., "Explain this as a friendly, talkative assistant").
Context management: When working with large datasets (e.g., entire books, codebases, or long videos), place your specific instructions or questions at the end of the prompt, after the data context. Anchor the model's reasoning to the provided data by starting your question with a phrase like, "Based on the information above...".
Learn more about prompt design strategies in the prompt engineering guide.

FAQ
What is the knowledge cutoff for Gemini 3 Pro? Gemini 3 has a knowledge cutoff of January 2025. For more recent information, use the Search Grounding tool.

What are the context window limits? Gemini 3 Pro supports a 1 million token input context window and up to 64k tokens of output.

Is there a free tier for Gemini 3 Pro? You can try the model for free in Google AI Studio, but currently, there is no free tier available for gemini-3-pro-preview in the Gemini API.

Will my old thinking_budget code still work? Yes, thinking_budget is still supported for backward compatibility, but we recommend migrating to thinking_level for more predictable performance. Do not use both in the same request.

Does Gemini 3 support the Batch API? Yes, Gemini 3 supports the Batch API.

Is Context Caching supported? Yes, Context Caching is supported for Gemini 3. The minimum token count required to initiate caching is 2,048 tokens.

Which tools are supported in Gemini 3? Gemini 3 supports Google Search, File Search, Code Execution, and URL Context. It also supports standard Function Calling for your own custom tools. Please note that Google Maps and Computer Use are currently not supported.

Next steps
Get started with the Gemini 3 Cookbook
Check the dedicated Cookbook guide on thinking levels and how to migrate from thinking budget to thinking levels.
Was this helpful?

Send feedback
Except as otherwise noted, the content of this page is licensed under the Creative Commons Attribution 4.0 License, and code samples are licensed under the Apache 2.0 License. For details, see the Google Developers Site Policies. Java is a registered trademark of Oracle and/or its affiliates.

Last updated 2025-11-20 UTC.

Terms
Privacy

English

Skip to main content
Gemini API
Search
/


English
Get API key
Cookbook
Community
Sign in
Gemini API Docs
API reference

Gemini 3 is here. Read the developer guide to get started with our most advanced model yet.
Home
Gemini API
Gemini API Docs
Was this helpful?

Send feedbackOpenAI compatibility

content_copy




Gemini models are accessible using the OpenAI libraries (Python and TypeScript / Javascript) along with the REST API, by updating three lines of code and using your Gemini API key. If you aren't already using the OpenAI libraries, we recommend that you call the Gemini API directly.

Python
JavaScript
REST

curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
-H "Content-Type: application/json" \
-H "Authorization: Bearer GEMINI_API_KEY" \
-d '{
    "model": "gemini-2.0-flash",
    "messages": [
        {"role": "user", "content": "Explain to me how AI works"}
    ]
    }'
What changed? Just three lines!

api_key="GEMINI_API_KEY": Replace "GEMINI_API_KEY" with your actual Gemini API key, which you can get in Google AI Studio.

base_url="https://generativelanguage.googleapis.com/v1beta/openai/": This tells the OpenAI library to send requests to the Gemini API endpoint instead of the default URL.

model="gemini-2.5-flash": Choose a compatible Gemini model

Thinking
Gemini 3 and 2.5 models are trained to think through complex problems, leading to significantly improved reasoning. The Gemini API comes with thinking parameters which give fine grain control over how much the model will think.

Gemini 3 uses "low" and "high" thinking levels, and Gemini 2.5 models use exact thinking budgets. These map to OpenAI's reasoning efforts as follows:

reasoning_effort (OpenAI)	thinking_level (Gemini 3)	thinking_budget (Gemini 2.5)
minimal	low	1,024
low	low	1,024
medium	high	8,192
high	high	24,576
If no reasoning_effort is specified, Gemini uses the model's default level or budget.

If you want to disable thinking, you can set reasoning_effort to "none" for 2.5 models. Reasoning cannot be turned off for Gemini 2.5 Pro or 3 models.

Python
JavaScript
REST

curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
-H "Content-Type: application/json" \
-H "Authorization: Bearer GEMINI_API_KEY" \
-d '{
    "model": "gemini-2.5-flash",
    "reasoning_effort": "low",
    "messages": [
        {"role": "user", "content": "Explain to me how AI works"}
      ]
    }'
Gemini thinking models also produce thought summaries. You can use the extra_body field to include Gemini fields in your request.

Note that reasoning_effort and thinking_level/thinking_budget overlap functionality, so they can't be used at the same time.

Python
JavaScript
REST

curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
-H "Content-Type: application/json" \
-H "Authorization: Bearer GEMINI_API_KEY" \
-d '{
    "model": "gemini-2.5-flash",
      "messages": [{"role": "user", "content": "Explain to me how AI works"}],
      "extra_body": {
        "google": {
           "thinking_config": {
             "include_thoughts": true
           }
        }
      }
    }'
Gemini 3 supports OpenAI compatibility for thought signatures in chat completion APIs. You can find the full example on the thought signatures page.

Streaming
The Gemini API supports streaming responses.

Python
JavaScript
REST

curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
-H "Content-Type: application/json" \
-H "Authorization: Bearer GEMINI_API_KEY" \
-d '{
    "model": "gemini-2.0-flash",
    "messages": [
        {"role": "user", "content": "Explain to me how AI works"}
    ],
    "stream": true
  }'
Function calling
Function calling makes it easier for you to get structured data outputs from generative models and is supported in the Gemini API.

Python
JavaScript
REST

curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
-H "Content-Type: application/json" \
-H "Authorization: Bearer GEMINI_API_KEY" \
-d '{
  "model": "gemini-2.0-flash",
  "messages": [
    {
      "role": "user",
      "content": "What'\''s the weather like in Chicago today?"
    }
  ],
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "description": "Get the current weather in a given location",
        "parameters": {
          "type": "object",
          "properties": {
            "location": {
              "type": "string",
              "description": "The city and state, e.g. Chicago, IL"
            },
            "unit": {
              "type": "string",
              "enum": ["celsius", "fahrenheit"]
            }
          },
          "required": ["location"]
        }
      }
    }
  ],
  "tool_choice": "auto"
}'
Image understanding
Gemini models are natively multimodal and provide best in class performance on many common vision tasks.

Python
JavaScript
REST
bash -c '
  base64_image=$(base64 -i "Path/to/agi/image.jpeg");
  curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer GEMINI_API_KEY" \
    -d "{
      \"model\": \"gemini-2.0-flash\",
      \"messages\": [
        {
          \"role\": \"user\",
          \"content\": [
            { \"type\": \"text\", \"text\": \"What is in this image?\" },
            {
              \"type\": \"image_url\",
              \"image_url\": { \"url\": \"data:image/jpeg;base64,${base64_image}\" }
            }
          ]
        }
      ]
    }"
'
Generate an image
Note: Image generation is only available in the paid tier.
Generate an image:

Python
JavaScript
REST
curl "https://generativelanguage.googleapis.com/v1beta/openai/images/generations" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer GEMINI_API_KEY" \
  -d '{
        "model": "imagen-3.0-generate-002",
        "prompt": "a portrait of a sheepadoodle wearing a cape",
        "response_format": "b64_json",
        "n": 1,
      }'
Audio understanding
Analyze audio input:

Python
JavaScript
REST
Note: If you get an Argument list too long error, the encoding of your audio file might be too long for curl.
bash -c '
  base64_audio=$(base64 -i "/path/to/your/audio/file.wav");
  curl "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions" \
    -H "Content-Type: application/json" \
    -H "Authorization: Bearer GEMINI_API_KEY" \
    -d "{
      \"model\": \"gemini-2.0-flash\",
      \"messages\": [
        {
          \"role\": \"user\",
          \"content\": [
            { \"type\": \"text\", \"text\": \"Transcribe this audio file.\" },
            {
              \"type\": \"input_audio\",
              \"input_audio\": {
                \"data\": \"${base64_audio}\",
                \"format\": \"wav\"
              }
            }
          ]
        }
      ]
    }"
'
Structured output
Gemini models can output JSON objects in any structure you define.

Python
JavaScript
from pydantic import BaseModel
from openai import OpenAI

client = OpenAI(
    api_key="GEMINI_API_KEY",
    base_url="https://generativelanguage.googleapis.com/v1beta/openai/"
)

class CalendarEvent(BaseModel):
    name: str
    date: str
    participants: list[str]

completion = client.beta.chat.completions.parse(
    model="gemini-2.0-flash",
    messages=[
        {"role": "system", "content": "Extract the event information."},
        {"role": "user", "content": "John and Susan are going to an AI conference on Friday."},
    ],
    response_format=CalendarEvent,
)

print(completion.choices[0].message.parsed)
Embeddings
Text embeddings measure the relatedness of text strings and can be generated using the Gemini API.

Python
JavaScript
REST
curl "https://generativelanguage.googleapis.com/v1beta/openai/embeddings" \
-H "Content-Type: application/json" \
-H "Authorization: Bearer GEMINI_API_KEY" \
-d '{
    "input": "Your text string goes here",
    "model": "gemini-embedding-001"
  }'
Batch API
You can create batch jobs, submit them, and check their status using the OpenAI library.

You'll need to prepare the JSONL file in OpenAI input format. For example:

{"custom_id": "request-1", "method": "POST", "url": "/v1/chat/completions", "body": {"model": "gemini-2.5-flash", "messages": [{"role": "user", "content": "Tell me a one-sentence joke."}]}}
{"custom_id": "request-2", "method": "POST", "url": "/v1/chat/completions", "body": {"model": "gemini-2.5-flash", "messages": [{"role": "user", "content": "Why is the sky blue?"}]}}
OpenAI compatibility for Batch supports creating a batch, monitoring job status, and viewing batch results.

Compatibility for upload and download is currently not supported. Instead, the following example uses the genai client for uploading and downloading files, the same as when using the Gemini Batch API.

Python
from openai import OpenAI

# Regular genai client for uploads & downloads
from google import genai
client = genai.Client()

openai_client = OpenAI(
    api_key="GEMINI_API_KEY",
    base_url="https://generativelanguage.googleapis.com/v1beta/openai/"
)

# Upload the JSONL file in OpenAI input format, using regular genai SDK
uploaded_file = client.files.upload(
    file='my-batch-requests.jsonl',
    config=types.UploadFileConfig(display_name='my-batch-requests', mime_type='jsonl')
)

# Create batch
batch = openai_client.batches.create(
    input_file_id=batch_input_file_id,
    endpoint="/v1/chat/completions",
    completion_window="24h"
)

# Wait for batch to finish (up to 24h)
while True:
    batch = client.batches.retrieve(batch.id)
    if batch.status in ('completed', 'failed', 'cancelled', 'expired'):
        break
    print(f"Batch not finished. Current state: {batch.status}. Waiting 30 seconds...")
    time.sleep(30)
print(f"Batch finished: {batch}")

# Download results in OpenAI output format, using regular genai SDK
file_content = genai_client.files.download(file=batch.output_file_id).decode('utf-8')

# See batch_output JSONL in OpenAI output format
for line in file_content.splitlines():
    print(line)    
The OpenAI SDK also supports generating embeddings with the Batch API. To do so, switch out the create method's endpoint field for an embeddings endpoint, as well as the url and model keys in the JSONL file:

# JSONL file using embeddings model and endpoint
# {"custom_id": "request-1", "method": "POST", "url": "/v1/embeddings", "body": {"model": "ggemini-embedding-001", "messages": [{"role": "user", "content": "Tell me a one-sentence joke."}]}}
# {"custom_id": "request-2", "method": "POST", "url": "/v1/embeddings", "body": {"model": "gemini-embedding-001", "messages": [{"role": "user", "content": "Why is the sky blue?"}]}}

# ...

# Create batch step with embeddings endpoint
batch = openai_client.batches.create(
    input_file_id=batch_input_file_id,
    endpoint="/v1/embeddings",
    completion_window="24h"
)
See the Batch embedding generation section of the OpenAI compatibility cookbook for a complete example.

extra_body
There are several features supported by Gemini that are not available in OpenAI models but can be enabled using the extra_body field.

extra_body features

cached_content	Corresponds to Gemini's GenerateContentRequest.cached_content.
thinking_config	Corresponds to Gemini's ThinkingConfig.
cached_content
Here's an example of using extra_body to set cached_content:

Python
from openai import OpenAI

client = OpenAI(
    api_key=MY_API_KEY,
    base_url="https://generativelanguage.googleapis.com/v1beta/"
)

stream = client.chat.completions.create(
    model="gemini-2.5-pro",
    n=1,
    messages=[
        {
            "role": "user",
            "content": "Summarize the video"
        }
    ],
    stream=True,
    stream_options={'include_usage': True},
    extra_body={
        'extra_body':
        {
            'google': {
              'cached_content': "cachedContents/0000aaaa1111bbbb2222cccc3333dddd4444eeee"
          }
        }
    }
)

for chunk in stream:
    print(chunk)
    print(chunk.usage.to_dict())
List models
Get a list of available Gemini models:

Python
JavaScript
REST
curl https://generativelanguage.googleapis.com/v1beta/openai/models \
-H "Authorization: Bearer GEMINI_API_KEY"
Retrieve a model
Retrieve a Gemini model:

Python
JavaScript
REST
curl https://generativelanguage.googleapis.com/v1beta/openai/models/gemini-2.0-flash \
-H "Authorization: Bearer GEMINI_API_KEY"
Current limitations
Support for the OpenAI libraries is still in beta while we extend feature support.

If you have questions about supported parameters, upcoming features, or run into any issues getting started with Gemini, join our Developer Forum.

What's next
Try our OpenAI Compatibility Colab to work through more detailed examples.

Was this helpful?

Send feedback
Except as otherwise noted, the content of this page is licensed under the Creative Commons Attribution 4.0 License, and code samples are licensed under the Apache 2.0 License. For details, see the Google Developers Site Policies. Java is a registered trademark of Oracle and/or its affiliates.

Last updated 2025-11-18 UTC.

Terms
Privacy

English
