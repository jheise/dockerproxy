dockerproxy
===========

dockerproxy scans docker for any running container that has exposed ports and the environment variable FORWARD=YES. Once containers are found ports are agregated together and a config file is generated for haproxy, if no containers are active a default config is used. dockerproxy should ideally be run from a docker container itself with the -net host option enabled.
