ARG VERSION

FROM golang:1.22 AS build

COPY . /src
RUN cd /src && go build -ldflags="-X 'github.com/plumber-cd/argocd-cmp-replicator/cmd/version.Version=$VERSION'" -o /bin/argocd-cmp-replicator

FROM ubuntu:latest

COPY plugin.yaml /home/argocd/cmp-server/config/plugin.yaml
RUN chmod +r /home/argocd/cmp-server/config/plugin.yaml
COPY --from=build /bin/argocd-cmp-replicator /usr/local/bin/argocd-cmp-replicator

RUN useradd -s /bin/bash -u 999 argocd
WORKDIR /home/argocd
USER argocd

ENTRYPOINT ["/usr/local/bin/argocd-cmp-replicator"]