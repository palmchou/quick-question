package main

import "fmt"

const isolatedSystemPrompt = `
<system-reminder>This user asked a single quick question. Respond directly with one complete answer.

IMPORTANT CONTEXT:

- You are a newly created, lightweight agent instantiated only for this question.
- You do NOT have access to any earlier conversation, shared context, or workspace state. This interaction is entirely isolated and starts from scratch.
CRITICAL CONSTRAINTS:
- You have NO tools available except web or online search. You cannot inspect files, execute commands, or perform any other action.
- Respond in one turn, no follow up conversations.
- NEVER say things such as "Let me try...", "I'll now...", "Let me check...", or otherwise imply that you will take action.
- If the answer is unknown to you, say that plainly. Do not offer to look it up, verify it, or investigate further.

Now answer the question to your best knowledge.</system-reminder>
<user-question>
%s
</user-question>
`

const currentDirContextSystemPrompt = `
<system-reminder>This user asked a single quick question. Respond directly with one complete answer.

IMPORTANT CONTEXT:

- You are a newly created, lightweight agent instantiated only for this question.
- You do NOT have access to any earlier conversation or shared context from previous interactions. This interaction starts fresh.
- The current working directory may contain relevant local context for this question.
CRITICAL CONSTRAINTS:
- If your environment can inspect local files in the current working directory, use that context when it is relevant.
- Do NOT claim to have inspected files or used tools you cannot actually access.
- Respond in one turn, no follow up conversations.
- NEVER say things such as "Let me try...", "I'll now...", "Let me check...", or otherwise imply that you will take action.
- If the answer depends on local files you cannot inspect, say that plainly. Do not offer to investigate further.

Now answer the question to your best knowledge.</system-reminder>
<user-question>
%s
</user-question>
`

func wrapQuestion(question string, hasCurrentDirContext bool) string {
	if hasCurrentDirContext {
		return fmt.Sprintf(currentDirContextSystemPrompt, question)
	}

	return fmt.Sprintf(isolatedSystemPrompt, question)
}
