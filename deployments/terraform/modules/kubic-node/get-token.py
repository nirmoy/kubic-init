#!/usr/bin/env python

# generate a kubeadm-compatible token

import json
import os
import sys


token = None
if len(sys.argv) > 1:
    token = sys.argv[1].strip()
else:
    token = os.environ.get('TOKEN')

if not token:
    try:
        import random
        token = "%0x.%0x" % (random.SystemRandom().getrandbits(3*8),
                             random.SystemRandom().getrandbits(8*8))
    except:
        token = ""

print(json.dumps({'token': token.strip()}, indent=2))
