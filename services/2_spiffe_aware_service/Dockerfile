FROM registry.scontain.com:5050/sconecuratedimages/spire:python3-network-shield-experimental

RUN pip install flask requests && mkdir /app

COPY ./spiffe_service.py /app/spiffe_service.py
