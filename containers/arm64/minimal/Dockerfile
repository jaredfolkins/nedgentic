# Minimal Ned guest: Debian slim, a shell, git, and a tiny browser desktop
# (ned-desktop). No agent CLIs are baked in: ned-agent-install fetches each one
# with its official installer when you choose its sign-in (ned-agent-login),
# and hooks/03-agent-updates.sh reinstalls signed-in CLIs and updates installed
# ones in the background on each boot.
# Everything here is open-source Debian packages plus Ned's own MIT scripts.
FROM --platform=linux/arm64 docker.io/library/debian:trixie-slim

LABEL org.opencontainers.image.source="https://github.com/jaredfolkins/nedgentic"

USER root

ARG SANDBOX_NAME="minimal-sandbox"
ENV SANDBOX="$SANDBOX_NAME"
ENV DEBIAN_FRONTEND=noninteractive

# One layer: skip docs, man pages, and non-English locales (license files
# stay), install the shell tools and the desktop (Xvfb + JWM + Chromium +
# xterm, served by x11vnc on loopback through noVNC/websockify on port 6080),
# then drop apt caches. noVNC's web client is static files; take them from
# Debian's novnc package without its Node.js dependency (~100 MB).
RUN set -eux; \
  printf '%s\n' \
    'path-exclude=/usr/share/doc/*' \
    'path-include=/usr/share/doc/*/copyright' \
    'path-exclude=/usr/share/man/*' \
    'path-exclude=/usr/share/info/*' \
    'path-exclude=/usr/share/lintian/*' \
    'path-exclude=/usr/share/locale/*' \
    'path-include=/usr/share/locale/en*' \
    'path-include=/usr/share/locale/locale.alias' \
    >/etc/dpkg/dpkg.cfg.d/ned-excludes; \
  find /usr/share/doc -mindepth 1 ! -type d ! -name copyright -delete; \
  find /usr/share/doc -mindepth 1 -type d -empty -delete; \
  rm -rf /usr/share/man/* /usr/share/info/* /usr/share/lintian/*; \
  find /usr/share/locale -mindepth 1 -maxdepth 1 ! -name 'en*' ! -name locale.alias -exec rm -rf {} +; \
  apt-get update; \
  apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    git \
    procps \
    sudo \
    chromium \
    fonts-dejavu-core \
    jwm \
    python3-websockify \
    x11vnc \
    xterm \
    xvfb; \
  cd /tmp; \
  apt-get download novnc; \
  dpkg-deb -x novnc_*.deb /tmp/novnc; \
  cp -a /tmp/novnc/usr/share/novnc /usr/share/novnc; \
  install -D -m 644 /tmp/novnc/usr/share/doc/novnc/copyright /usr/share/doc/novnc/copyright; \
  rm -rf /tmp/novnc /tmp/novnc_*.deb; \
  test -f /usr/share/novnc/vnc.html; \
  apt-get clean; \
  rm -rf /var/lib/apt/lists/* /var/cache/apt/* /var/cache/debconf/*-old /var/log/apt /var/log/dpkg.log; \
  install -d -m 1777 /tmp/.X11-unix; \
  install -d -m 755 /opt/ned-agents

# Agent installer and updater, the desktop launcher and its window manager
# config, and the gentic hooks.
COPY --chmod=755 scripts/ned-agent-install scripts/ned-agent-update scripts/ned-desktop /usr/local/bin/
COPY --chmod=644 scripts/ned-desktop-jwmrc /etc/ned-desktop/jwmrc
COPY --chmod=755 hooks/*.sh /ned/hooks/
CMD ["bash"]
