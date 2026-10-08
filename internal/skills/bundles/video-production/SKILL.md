# Video production
Coordinate a complete video task. Read this skill before producing a video; then read the relevant specialist skills with the skill tool.

## Brief and preferences
Ask only for missing decisions: topic/audience, target duration, language, aspect ratio, preferred image/video website (Gemini, ChatGPT, or another), narration/voice, editor (FFmpeg default for low resources, CapCut if preferred), and intended platforms. Offer reasonable defaults: 30 seconds, 720p, 30 fps, no narration unless requested. Do not change a chosen provider without asking. Save confirmed preferences and brief.json under the agent's project folder; reuse them in follow-up requests.
Distinguish generated moving footage from a slideshow of generated stills. Explain the chosen approach. Verify the preferred site actually offers video generation for this signed-in account. Ask before paid generation, subscriptions, or uploading sensitive source material.

## Production
1. Create a project under /workspace/agents/<agent-id>/video-projects/<unique-project>/ with assets/, output/, and brief.json, storyboard.md, manifest.json, timeline.json, progress.json. Do not overwrite an earlier deliverable.
2. Write a scene plan with timing, visual prompts, narration and transitions; keep subject/style consistent. Read browser-media-generation before generating assets. Download each completed asset immediately; record its path, provider, prompt, source and duration in manifest.json. Preserve useful results and resume from progress.json rather than regenerate everything.
3. Read transcription-subtitles for narration/transcripts/captions. Avoid fabricated audio transcripts or unverified timestamps. Provide a script if there is no spoken audio.
4. Read ffmpeg-video-editing for local assembly or capcut-browser-editing for browser assembly. Use one render at a time, low thread counts, and a short preview before the full render. Do not install large ML/editor packages by default.
5. Check the final duration, audio, orientation, first/middle/last frames and caption alignment. Save final MP4, thumbnail, transcript, captions, editable timeline, and a short README listing any omissions. Keep interim caches out of output/.
6. Link final files in the response. If publishing is requested, read social-publishing and prepare platform-specific metadata. Generation alone is not permission to publish.

## Continuing across messages
Checkpoint after each scene and before long waits. If the model round budget is low, save finished files and state exactly which stage remains. On continue, inspect progress.json and existing files first. Keep chat updates brief: stage, completed result, actionable blocker.
