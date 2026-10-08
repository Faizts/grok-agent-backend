# CapCut browser editing
Use when the user prefers CapCut or needs browser timeline editing. FFmpeg remains the low-resource default when no editor is selected.

1. Open the current CapCut web editor and inspect its actual import/export controls. Let the user sign in manually through Shared Computer. Verify the selected account supports web editing and required features. Never assume premium features are free; ask before paying or changing tools.
2. Read storyboard, assets manifest and actual media files. Create a uniquely named project. Import files with browser upload on observed input[type=file] controls; use the file tool to list paths first. Confirm each upload completes and appears in the media bin.
3. Assemble scenes in storyboard order and confirm durations on the timeline. Set the requested canvas ratio; add approved narration/music, text and verified captions. If reliable drag/drop controls are unavailable, inspect supported DOM controls or ask the user for the specific manual edit; never claim an unseen timeline was edited.
4. Preview beginning, transitions, middle and ending. Check audio/caption timing and clipping. Export the selected resolution/format using only authorized options; wait for progress without repeated export submissions.
5. Capture the real export through browser download_click into project/output/final.mp4. Validate format, duration, size and dimensions with ffprobe. Preserve the editable project URL in README without credentials. A project URL is not a downloaded final video.
6. If export/login/features block progress, save assets, manifest and completed steps; offer FFmpeg fallback for user selection. Never lose completed work or publish it automatically.
Reference: https://www.capcut.com/tools/online-video-editor
