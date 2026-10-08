// Package agent runs serialized, restorable conversations. Configure supplies the
// search endpoint and optional embedder; SetHistory restores persisted messages
// including assistant tool calls and matching tool_call_id result messages.
// Set ConversationID before running to associate usage with the conversation.
//
// Dangerous tools fail closed without an approvals store. Approval events expose
// approval_id for polling decisions and tool_call_id for UI/persistence pairing.
// Pending approvals expire after five minutes. Only file read/list and web_search
// are automatic; other tools require approval or an explicit always_allow rule.
//
// Billing requires users.budget_period and model_rates, and an OpenAI-compatible
// provider supporting stream_options.include_usage. Configure monetary columns
// with enough precision to preserve sub-cent token costs. MCP supports Streamable
// HTTP only; stdio is deliberately not executed on the application host.
package agent
