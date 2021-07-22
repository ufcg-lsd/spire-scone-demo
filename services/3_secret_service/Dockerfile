FROM registry.scontain.com:5050/sconecuratedimages/spire:network-shield-python-alpha3

RUN pip install flask && mkdir /app && echo '{"secret": "Eve is good and Bob is the bad guy"}' > /app/index.html

COPY ./secret_service.py /app/secret_service.py
