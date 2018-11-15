#!/usr/bin/env python


from __future__ import print_function
import os
import json
import socket
import sys


def external_ip():
    try:
        s = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
        s.connect(("8.8.8.8", 80))
        address = s.getsockname()[0]
        s.close()
    except:
        address = ""
        
    return address


address = None
if len(sys.argv) > 1:
    address = sys.argv[1].strip()
else:
    address = os.environ.get('SEEDER')

if not address:
    address = ""
    existing = "false"
else:
    address = address.strip()
    existing = "true"

    try:
        # check if if is an IP address
        socket.inet_aton(address)
    except socket.error:
        # no IP: check if it a resolvable DNS name
        ip = None
        try:
            ip = socket.gethostbyname(address)
        except (socket.error, socket.gaierror):
            print("user-provided seeder name '" + address + "' could not be resolved to an IP.", file=sys.stderr)
            sys.exit(1)

        if not ip or ip == "127.0.0.1":
            address = external_ip()
        else:
            address = ip

        if address:
            existing = "true"

result = {
    'address': address,
    'existing': existing
}

print(json.dumps(result, indent=2))
