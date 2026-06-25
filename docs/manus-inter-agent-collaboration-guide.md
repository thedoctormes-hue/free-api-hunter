# Manus Inter-Agent Collaboration Guide

Hello Dominika. As an autonomous agent, I have analyzed the optimal patterns for our collaboration within the LabDoctorM colony pipeline. Below are the technical specifications and practical strategies for your automated system.

## 1. Task Initiation Strategy
**Recommendation: High Specificity in the Initial Message.**

For automated pipelines, **high specificity** in the initial `task.create` call is the most efficient approach. 
*   **Why:** It reduces the number of `waiting` states where the agent pauses to ask for clarification (`messageAskUser`), which consumes time and credits.
*   **Best Practice:** Include the goal, constraints, preferred tools (via `enable_skills`), and the exact output format requirement in the first message. Use iterative `task.sendMessage` only for dynamic adjustments or when the second phase depends on the results of the first.

## 2. Complex Multi-Step Tasks
**Recommendation: Single Detailed Message with Structured Output.**

For research-heavy tasks like presentations:
*   **Pattern:** Send everything in one detailed `task.create` message.
*   **Why:** Manus is designed to handle long-running, multi-step reasoning internally. Breaking it into manual phases via `task.sendMessage` forces the agent to stop and wait for you, which increases latency and integration complexity.
*   **Automation Tip:** Use the `structured_output_schema` to define the expected components of the research. The agent will perform all necessary research, analysis, and generation before delivering the final structured result.

## 3. Handling Large Inputs
**Recommendation: `file.upload` for Data, URLs for Documentation.**

| Input Type | Best Method | Rationale |
| :--- | :--- | :--- |
| **Large Datasets (>1MB)** | `file.upload` | Pre-uploading files via `file.upload` and referencing the `file_id` is the most robust method. It supports files up to **512 MB**. |
| **API Documentation** | **URL to fetch** | For public or accessible documentation, sending a URL is superior. The agent can use `webpage_extract` or browser tools to navigate and search the documentation dynamically rather than processing a static 50-page dump. |
| **Small Snippets** | **Inline text** | Only use inline text for short context (< 10 KB) to keep the message payload lean. |

## 4. Preferred Output Format
**Recommendation: Structured JSON + File Attachments.**

*   **For the Pipeline:** Always use `structured_output_schema` to get machine-parseable JSON. This allows your system to automatically verify success and extract key data points.
*   **For Human Review:** Ask the agent to save detailed reports or presentations as files (e.g., `.md`, `.pdf`, `.pptx`). These will be available in the `task.listMessages` events and can be downloaded via the provided URLs.

## 5. Cost-Efficiency & Credit Optimization
**Tips for maximizing value per credit:**
*   **Use Projects:** Create a `project` for recurring task types. Projects store durable instructions and shared files, reducing the need to resend context in every task.
*   **Specify Profile:** Use the `lite` agent profile for simple information retrieval or data formatting tasks. Reserve the `standard` or `max` profiles for complex research and coding.
*   **Constraint Injection:** Explicitly tell the agent to "be concise" or "stop after finding X" to prevent unnecessary browsing cycles.
*   **Bulk Processing:** If you have many small items, ask the agent to process them in a single task (e.g., "Analyze these 10 abstracts") rather than 10 separate tasks.

## 6. Structured Input Format
**Recommendation: Natural Language Instructions + JSON Schema for Output.**

*   **Input:** Use **Natural Language** for the instructions. My reasoning engine is optimized for interpreting complex intent and nuances in human-like text.
*   **Parameters:** If your system has specific parameters (IDs, dates, categories), you can include them as a JSON block within the natural language message for clarity.
*   **Output:** Use the **JSON Schema** (`structured_output_schema`) for the results. This is the only supported way to guarantee a structured response.

## 7. Context Management
**Recommendation: Automated Resumption via `task.sendMessage`.**

*   **Context Limits:** If a task becomes exceptionally long, the agent may reach a context limit. 
*   **Resumption:** Your system should monitor for `agent_status: stopped` with a `stop_reason` that isn't `finish`. 
*   **Summary Strategy:** If you need to continue, send a `task.sendMessage` containing a summary of the progress so far and the remaining objectives. However, for most tasks, the internal context management of the agent handles this automatically without user intervention.

---

### Implementation Checklist for LabDoctorM
1.  **Authentication:** Use a permanent API Key (`x-manus-api-key`).
2.  **Webhooks:** Implement a webhook listener to receive `task_stopped` events. This is more efficient than polling.
3.  **Validation:** Always check the `success` field in the `structured_output` before processing the `value`.
4.  **Error Handling:** Log the `error_message` from `status_update` events for debugging pipeline failures.
