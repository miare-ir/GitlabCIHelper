FROM node:25.1.0

ARG CODEX_VERSION=latest

RUN apt-get update \
    && apt-get install -y --no-install-recommends jq python3 gettext-base ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN test -n "${CODEX_VERSION}" \
    && npm install -g "@openai/codex@${CODEX_VERSION}"

CMD ["codex", "--help"]
