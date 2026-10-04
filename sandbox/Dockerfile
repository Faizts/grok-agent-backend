FROM ubuntu:22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV DISPLAY=:1
ENV PLAYWRIGHT_BROWSERS_PATH=/opt/playwright-browsers

# Base system + desktop
RUN apt-get update && apt-get install -y \
    python3.11 python3-pip python3-venv \
    curl wget git vim nano jq unzip \
    xfce4 xfce4-terminal \
    xvfb x11vnc \
    supervisor \
    nodejs npm \
    libnss3 libatk1.0-0 libatk-bridge2.0-0 libcups2 \
    libdrm2 libxkbcommon0 libxcomposite1 libxdamage1 \
    libxfixes3 libxrandr2 libgbm1 libasound2 \
    libxss1 libgconf-2-4 libx11-xcb1 \
    && rm -rf /var/lib/apt/lists/*

# Install noVNC
RUN git clone --depth 1 https://github.com/novnc/noVNC.git /opt/novnc \
    && git clone --depth 1 https://github.com/novnc/websockify.git /opt/novnc/utils/websockify \
    && ln -s /opt/novnc/vnc.html /opt/novnc/index.html

# Python packages
RUN pip3 install --no-cache-dir \
    playwright \
    numpy pandas matplotlib scipy \
    requests beautifulsoup4 lxml \
    pillow \
    ipython

# Playwright browsers
RUN python3 -m playwright install chromium \
    && python3 -m playwright install-deps chromium

WORKDIR /workspace

COPY supervisord.conf /etc/supervisor/conf.d/supervisord.conf
COPY start.sh /start.sh
RUN chmod +x /start.sh

# Persistent workspace volume
VOLUME ["/workspace"]

# VNC port, noVNC web port
EXPOSE 5900 6080

CMD ["/start.sh"]
