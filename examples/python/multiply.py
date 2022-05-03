#!/usr/local/bin/python

import os
from time import sleep
from progress.bar import ChargingBar

env = float(os.getenv("NUM"))
user = float(input("number: "))

for i in ChargingBar("Multiply", max=32, check_tty=False).iter(range(32)):
    sleep(0.1)

print(f"Done! {env} * {user} = {env * user}")
