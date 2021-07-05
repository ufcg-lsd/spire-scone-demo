from flask import Flask
import requests
import os
import json


app = Flask(__name__)
secret_service_url = None
ca_path = None


def get_env(name):
    e = os.environ.get(name, None)
    if e is None:
        raise Exception('Cannot get env var {}'.format(name))
    return e


@app.route("/")
def get_counter():
    try:
        resp = requests.get(secret_service_url, verify=ca_path)
        print('response: {}'.format(resp.text))
        return resp.json(), 200
    except Exception as e:
        return json.dumps({'error': str(e)}), 500


if __name__ == '__main__':
    cert_path = get_env('SVID_CERT_PATH')
    key_path = get_env('SVID_KEY_PATH')
    ca_path = get_env('BUNDLE_PATH')
    secret_service_url = get_env('SECRET_SERVICE_URL')

    with open(cert_path, 'r') as f:
        cert = f.read()
        print('SVID Chain\n{}'.format(cert))
    
    with open(ca_path, 'r') as f:
        ca = f.read()
        print('CA\n{}'.format(ca))

    app.run(host="0.0.0.0", ssl_context=(cert_path, key_path))
