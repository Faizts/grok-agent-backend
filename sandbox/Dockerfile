FROM debian:bookworm-slim
LABEL io.grokagent.project="grokagent" io.grokagent.service="sandbox"
ENV DEBIAN_FRONTEND=noninteractive DISPLAY=:1 PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers
RUN apt-get update && apt-get install -y --no-install-recommends \
    ffmpeg python3 python3-pip python3-venv curl git ca-certificates \
    openbox xterm xvfb x11vnc supervisor novnc websockify fonts-dejavu-core \
    && rm -rf /var/lib/apt/lists/*
RUN python3 -m venv /opt/venv
ENV PATH=/opt/venv/bin:$PATH
ARG INSTALL_DATA_PACKAGES=false
RUN pip install --no-cache-dir playwright requests beautifulsoup4 pillow \
    && if [ "$INSTALL_DATA_PACKAGES" = "true" ]; then pip install --no-cache-dir numpy pandas matplotlib; fi \
    && python -m playwright install --with-deps --no-shell chromium \
    && python -c "from playwright.sync_api import sync_playwright; import os; p=sync_playwright().start(); os.symlink(p.chromium.executable_path, '/usr/local/bin/agent-chromium'); p.stop()" \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /workspace
COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY start.sh /start.sh
RUN chmod +x /start.sh
VOLUME ["/workspace"]
EXPOSE 5900 6080
CMD ["/start.sh"]
