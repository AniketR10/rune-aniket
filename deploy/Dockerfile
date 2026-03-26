# syntax=docker/dockerfile:1
FROM --platform=linux/amd64 golang:alpine as build

# git and openssh needed to go get private repos
RUN apk add --no-cache make git openssh build-base libx11-dev libxrandr-dev libva-glx-dev libxcursor-dev libxinerama-dev libxi-dev

ARG GIT_SSH_KEY
ARG GCP_PROJECT "unstable-build-blue-dev"

WORKDIR /src

COPY . .

RUN git config --global url.ssh://git@github.com/.insteadOf https://github.com/
RUN mkdir -p ~/.ssh && \
	echo "$GIT_SSH_KEY" > ~/.ssh/id_rsa && \
	chmod 600 ~/.ssh/id_rsa && \
	chmod 700 ~/.ssh
RUN ssh-keyscan -t rsa git.unstable.build >> ~/.ssh/known_hosts
RUN ssh-keyscan -t rsa github.com >> ~/.ssh/known_hosts
RUN go env -w GOPRIVATE="github.com/unstablebuild,unstable.build/*"

RUN make release && cp target/linux_amd64/ox-api /bin/ox-api

# use clean image with no deps or install
FROM --platform=linux/amd64 alpine:latest
COPY --from=build /bin/ox-api /bin/ox-api

EXPOSE 4433
EXPOSE 3001

CMD [ "sh", "-c", "/bin/ox-api -G 4433 -P 3001 -p ${GCP_PROJECT} -H 0.0.0.0 -I -J" ]
