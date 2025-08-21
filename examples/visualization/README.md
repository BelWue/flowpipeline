#Deployment example

The files in this directory are used to create a example Docker 
container that shows how Grafana to visualize flowpieline data.

Use `docker pull ghcr.io/ynhuber/flowpipeline-grafana:example-kafka-container` 
to get the container and 
`docker run -p 3000:3000 -p 2055:2055 -d ghcr.io/ynhuber/flowpipeline-grafana:example-kafka-container` 
to run the container on your local system.
This will start a netflow receiver on port 2055 as well as grafana on port 3000. 
You can then explore example dashboards at `http://localhost:3000/`.
The grafana login credentials are the default credentials `admin:admin`.

The data collected via the flow collector is processed using flowpipeline and then collected using prometheus.
Note: This is done using all 3 tools (grafana, prometheus & flowpipeline) in one container. 
This is only done for demonstration purposes. For a production deployment it is strongly advised to use the corresponding standalone containers.