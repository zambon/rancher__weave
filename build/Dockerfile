FROM ppc64le/golang:1.8.2

ENV GO15VENDOREXPERIMENT 1

RUN set -x && \
  apt-get -q update && apt-get -qy install \
    libpcap-dev \
    python-requests \
    time \
    file \
  && rm -rf /var/lib/apt/lists/*

RUN set -x && \
  mkdir -p $GOPATH/src/github.com/golang && \
  cd $GOPATH/src/github.com/golang && \
  git clone https://github.com/golang/lint.git && \
  cd lint && \
  git checkout 3390df4df2787994aea98de825b964ac7944b817 && \
  \
  mkdir -p $GOPATH/src/github.com/fzipp && \
  cd $GOPATH/src/github.com/fzipp && \
  git clone https://github.com/fzipp/gocyclo.git && \
  cd gocyclo && \
  git checkout 6acd4345c835499920e8426c7e4e8d7a34f1bb83 && \
  \
  mkdir -p $GOPATH/src/github.com/client9 && \
  cd $GOPATH/src/github.com/client9 && \
  git clone https://github.com/client9/misspell.git && \
  cd misspell && \
  git checkout 5cb2f5365e6061dc8e8f4bd6b4aadf553f1a015f

RUN set -x && \
  go get \
    github.com/golang/lint/golint \
    github.com/fzipp/gocyclo \
    github.com/client9/misspell/cmd/misspell

RUN go clean -i net && go install -tags netgo std
RUN go install -tags netgo std

COPY build.sh /

ENTRYPOINT ["sh", "/build.sh"]
