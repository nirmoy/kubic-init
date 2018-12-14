#!/usr/bin/env python

# generate a kubeadm-compatible token

import random
import json
import os

# use the TOKEN variable if preset
token = os.environ.get('TOKEN')
if token is None:
    a = "%0x" % random.SystemRandom().getrandbits(13*8)
    token = a[:6] + "." + a[6:22]

print(json.dumps({'token': token}, indent=2))
