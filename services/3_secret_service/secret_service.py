LISTEN_ADDR = '0.0.0.0'
LISTEN_PORT = 5000

import http.server
httpd = http.server.HTTPServer((LISTEN_ADDR, LISTEN_PORT), http.server.SimpleHTTPRequestHandler)
httpd.serve_forever()
