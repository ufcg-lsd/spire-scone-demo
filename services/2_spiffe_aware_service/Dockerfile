FROM registry.scontain.com:5050/sconecuratedimages/spire:network-shield-python-alpha3

RUN pip install flask requests && mkdir /app

COPY ./spiffe_service.py /app/spiffe_service.py
