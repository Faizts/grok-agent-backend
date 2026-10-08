# Transcription and subtitles
Use for scripts, voiceover transcripts, SRT and WebVTT captions.

## Determine the source
Ask whether the user wants a script before narration or a transcript of existing speech. A written script is not proof of what the recording actually says. For existing audio, inspect audio tracks/duration with ffprobe and extract mono audio with FFmpeg only if the selected service needs it.
Ask which transcription/voice service to use when unspecified. Prefer the user's already available browser service or provided transcript. Check whether the user permits sending the audio to that service. API use needs a separately configured credential; never request secrets in chat or record them in project manifests. Install/download a local speech model only if the user explicitly prefers local transcription and resources allow it. Do not silently fetch large models.

## Workflow
Upload authorized source audio through browser upload when the selected site offers transcription. Download transcript/timestamped output, then review ambiguous words, speaker names and the beginning/end against the recording. Mark uncertain words rather than invent them. Keep spoken text faithful; store edited narration scripts separately.
Write transcript.txt, captions.srt and captions.vtt in output/. Use ordered non-overlapping timestamps inside the actual recording duration, readable line lengths and at most two caption lines per cue. Preserve punctuation/language. Do not derive precise timings by merely distributing words evenly; obtain measured alignment from the service or verify manually. If timing cannot be verified, deliver the untimed transcript and explain that timed captions are unfinished.
Use only user-approved voice/music sources. Verify narration pronunciation and length before final assembly. Give the editing workflow the real audio path and caption files. Return downloadable transcript/captions and state the source and any uncertainty.
