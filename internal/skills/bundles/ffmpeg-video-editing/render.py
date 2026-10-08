#!/usr/bin/env python3
"""Sequential, bounded FFmpeg assembly. Python standard library only."""
import json
import math
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile


def render(timeline):
    timeline = Path(timeline).resolve()
    root = timeline.parent
    data = json.loads(timeline.read_text())

    def project_path(value):
        path = (root / value).resolve()
        if not path.is_relative_to(root) or path == root:
            raise ValueError('Media/output path escapes project')
        return path

    def run(args):
        subprocess.run(args, check=True, timeout=1200, stdout=subprocess.DEVNULL)

    w, h, fps = data.get('width', 1280), data.get('height', 720), data.get('fps', 30)
    if any(type(v) is not int for v in (w, h, fps)) or not (2 <= w <= 1920 and 2 <= h <= 1920 and w % 2 == h % 2 == 0 and 1 <= fps <= 30):
        raise ValueError('Invalid dimensions or fps')
    scenes = data['scenes']
    if not 1 <= len(scenes) <= 100:
        raise ValueError('Use 1–100 scenes')
    total = 0
    for scene in scenes:
        duration = float(scene['duration'])
        if not math.isfinite(duration) or not 0.1 <= duration <= 120:
            raise ValueError('Scene duration must be 0.1–120 seconds')
        total += duration
        if scene['kind'] not in ('image', 'video') or not project_path(scene['path']).is_file():
            raise ValueError('Missing asset or unsupported kind')
    if total > 600:
        raise ValueError('Render limited to 10 minutes')
    output = project_path(data.get('output', 'output/final.mp4'))
    if output.exists():
        raise ValueError('Output already exists; choose a new filename')
    output.parent.mkdir(parents=True, exist_ok=True)
    ffmpeg = shutil.which('ffmpeg')
    ffprobe = shutil.which('ffprobe')
    if not ffmpeg or not ffprobe:
        raise RuntimeError('ffmpeg and ffprobe are required')
    base = [ffmpeg, '-hide_banner', '-loglevel', 'error', '-nostdin', '-y', '-filter_threads', '1', '-filter_complex_threads', '1']
    codec = ['-c:v', 'libx264', '-preset', 'veryfast', '-crf', '25', '-pix_fmt', 'yuv420p', '-threads', '2']
    with tempfile.TemporaryDirectory(prefix='.render-', dir=root) as temp:
        temp = Path(temp)
        for i, scene in enumerate(scenes):
            inputs = ['-loop', '1'] if scene['kind'] == 'image' else ['-stream_loop', '-1']
            run(base + inputs + ['-i', str(project_path(scene['path'])), '-t', str(scene['duration']), '-vf',
                f'scale={w}:{h}:force_original_aspect_ratio=decrease,pad={w}:{h}:(ow-iw)/2:(oh-ih)/2,setsar=1,fps={fps}',
                '-an'] + codec + [str(temp / f'{i}.mp4')])
        (temp / 'clips.txt').write_text(''.join(f"file '{i}.mp4'\n" for i in range(len(scenes))))
        joined = temp / 'joined.mp4'
        run(base + ['-f', 'concat', '-safe', '1', '-i', str(temp / 'clips.txt'), '-c', 'copy', str(joined)])
        if data.get('audio'):
            audio = project_path(data['audio'])
            if not audio.is_file():
                raise ValueError('Missing narration')
            run(base + ['-i', str(joined), '-i', str(audio), '-map', '0:v:0', '-map', '1:a:0', '-c:v', 'copy',
                '-c:a', 'aac', '-b:a', '128k', '-af', 'apad', '-t', str(total), '-movflags', '+faststart', str(output)])
        else:
            run(base + ['-i', str(joined), '-c', 'copy', '-movflags', '+faststart', str(output)])
    probe = subprocess.check_output([ffprobe, '-v', 'error', '-show_format', '-show_streams', '-of', 'json', str(output)], timeout=30)
    metadata = json.loads(probe)
    video = next(s for s in metadata['streams'] if s['codec_type'] == 'video')
    if video['width'] != w or video['height'] != h or abs(float(metadata['format']['duration']) - total) > 0.5:
        raise RuntimeError('Export failed duration/dimensions verification')
    if data.get('audio') and not any(s['codec_type'] == 'audio' for s in metadata['streams']):
        raise RuntimeError('Export missing narration')
    output.with_suffix('.probe.json').write_bytes(probe)
    print(json.dumps({'saved': str(output), 'bytes': output.stat().st_size, 'duration': metadata['format']['duration']}))


if __name__ == '__main__':
    render(sys.argv[1])
