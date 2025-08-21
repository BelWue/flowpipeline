FROM golang:tip-trixie AS builder
# add local repo into the builder
ADD . /opt/build
WORKDIR /opt/build

RUN apt update &&\
    apt install libpcap-dev npm -y

# build the binary there
RUN ls -lha /opt/build
RUN go build -tags container -o fpl -v 

RUN git clone https://github.com/prometheus/prometheus.git
WORKDIR /opt/build/prometheus/cmd/prometheus
RUN git checkout v3.5.0
RUN GO111MODULE=on go build .

WORKDIR /opt/build/prometheus/
RUN make assets

# begin new container
FROM debian:trixie-slim

RUN apt update &&\
    apt install -y ca-certificates apt-transport-https wget gnupg2  libpcap-dev supervisor &&\
    mkdir -p /etc/apt/keyrings/ &&\
    wget -q -O - https://apt.grafana.com/gpg.key | gpg --dearmor | tee /etc/apt/keyrings/grafana.gpg > /dev/null &&\
    echo "deb [signed-by=/etc/apt/keyrings/grafana.gpg] https://apt.grafana.com stable main" | tee -a /etc/apt/sources.list.d/grafana.list &&\
    apt update &&\
    apt -y install grafana && \
    #Test - remove later
    apt -y install nfdump htop curl 


WORKDIR /
RUN mkdir -p /flowpipeline-example/{config,static}
WORKDIR /flowpipeline-example



# copy binaries from builder to your desired location
COPY --from=builder /opt/build/fpl .
COPY --from=builder /opt/build/prometheus/cmd/prometheus/prometheus .
COPY --from=builder /opt/build/prometheus/web/ui/static static

# add configuration files for all services
COPY examples/visualization/flowpipeline/flowpipeline-config.yml config/flowpipeline-config.yml
COPY examples/visualization/prometheus/prometheus-config.yml config/prometheus-config.yml

# add example grafana dashboards
COPY examples/visualization/grafana/grafana-datasource.yaml /usr/share/grafana/conf/provisioning/datasources/default.yml
COPY examples/visualization/grafana/grafana-dashboards.yaml /usr/share/grafana/conf/provisioning/dashboards/default.yml


COPY examples/visualization/grafana/dashboards/overview.json /flowpipeline-example/dashboards/overview.json
COPY examples/visualization/grafana/dashboards/raw-data.json /flowpipeline-example/dashboards/raw-data.json

# Start grafana, prometheus & flowpipeline 
ENTRYPOINT /usr/share/grafana/bin/grafana server --config /etc/grafana/grafana.ini \
--pidfile /run/grafana/grafana-server.pid --packaging deb --homepath /usr/share/grafana \
-- cfg:default.paths.logs=/var/log/grafana & \
/flowpipeline-example/prometheus --config.file=/flowpipeline-example/config/prometheus-config.yml & \
/flowpipeline-example/fpl -c /flowpipeline-example/config/flowpipeline-config.yml
