#!/usr/bin/env python

import socket

s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
s.connect(("8.8.8.8", 80))
ip = s.getsockname()[0]

s.close()

import json
print(json.dumps({'ip': ip}, indent=2))
