FROM golang:1.10

# install glide
RUN go get -v github.com/Masterminds/glide \
    && apt-get update \
    && apt-get install -y python-virtualenv \
    && rm -rf /var/lib/apt/lists/* \
    && cd $GOPATH/src/github.com/Masterminds/glide \
    && git checkout tags/0.10.2 \
    && go install \
    && cd -

COPY . $GOPATH/src/github.com/narmitech/cloudwatchmetricbeat
RUN cd $GOPATH/src/github.com/narmitech/cloudwatchmetricbeat && make collect && make && make update

RUN mkdir -p /etc/cloudwatchmetricbeat/ \
    && cp $GOPATH/src/github.com/narmitech/cloudwatchmetricbeat/cloudwatchmetricbeat /usr/local/bin/cloudwatchmetricbeat \
    && cp $GOPATH/src/github.com/narmitech/cloudwatchmetricbeat/_meta/beat.yml /etc/cloudwatchmetricbeat/cloudwatchmetricbeat.yml \
    && cp $GOPATH/src/github.com/narmitech/cloudwatchmetricbeat/cloudwatchmetricbeat.template.json /etc/cloudwatchmetricbeat/cloudwatchmetricbeat.template.json \
    && cp $GOPATH/src/github.com/narmitech/cloudwatchmetricbeat/cloudwatchmetricbeat.template-es2x.json /etc/cloudwatchmetricbeat/cloudwatchmetricbeat.template-es2x.json

WORKDIR /etc/cloudwatchmetricbeat
ENTRYPOINT cloudwatchmetricbeat
