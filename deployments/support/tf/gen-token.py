#!/usr/bin/env python

# generate a kubeadm-compatible token

import random
import json

token = "%0x.%0x" % (random.SystemRandom().getrandbits(3*8),
                     random.SystemRandom().getrandbits(8*8))

print(json.dumps({'token': token}, indent=2))
