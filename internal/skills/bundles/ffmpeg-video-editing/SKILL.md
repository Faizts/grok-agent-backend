# FFmpeg video editing
Use FFmpeg and Python standard library for low-resource local assembly. Avoid MoviePy, GPU dependencies, and large model downloads by default.

## Tools
Check ffmpeg -version and ffprobe -version. New computers include them. On older computers ask/use existing permitted package installation if absent; report a missing dependency instead of pretending a render succeeded.
Read the companion render.py with skill(name="ffmpeg-video-editing", resource="render.py") and save it in the project folder using file write. It supports stills/clips, cuts, uniform framing, optional narration, and MP4 export. Do not claim it supports crossfades or burning subtitles: extend the timeline/render explicitly when requested, or use CapCut. It replaces source clip audio with optional narration; preserve/mix source audio explicitly if requested.

## Timeline and execution
Create timeline.json with width/height (even, up to 1920), fps (up to 30), scenes [{path, kind: "image" or "video", duration: seconds}], optional audio path, output path. Paths are relative to the project folder and cannot escape it. Example: {"width":720,"height":1280,"fps":30,"scenes":[{"path":"assets/scene-1.jpg","kind":"image","duration":5}],"output":"output/final.mp4"}.
Run python3 render.py timeline.json from the project folder. Use a short preview to verify framing before rendering all scenes. Limit total duration to ten minutes per render and one FFmpeg process with two threads. Render clips sequentially, normalize codec/framerate/dimensions, concatenate, and use AAC narration if supplied. Keep originals. Export sidecar SRT/VTT or burn verified subtitles with FFmpeg's subtitles filter when available, safely escaping filter paths. For transitions use verified FFmpeg xfade parameters with matching timebases; do not add them without testing.

## Verification
ffprobe must show a valid H.264 MP4, expected duration and dimensions, and audio when requested. Sample first/middle/last frames and listen to short audio samples. Confirm captions align to the actual audio; include editable timeline and assets manifest. Remove only this project's temporary normalized clips after success, preserve outputs. Keep final downloadable files within the 50 MiB download limit or export smaller settings/split deliverables and explain.
Reference: https://www.ffmpeg.org/ffmpeg.html and https://www.ffmpeg.org/ffmpeg-filters.html
