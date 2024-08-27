FROM lscr.io/linuxserver/openssh-server:latest

COPY ./bin/six /usr/local/bin/six
COPY ./workspace/ssh/test/id_ed25519.pub /id_ed25519.pub
RUN chmod +x /usr/local/bin/six
