#!/usr/local/bin/python

from time import sleep
from progress.bar import ChargingBar

a = float(input("number a: "))
b = float(input("number b: "))

for i in ChargingBar("Multiply", max=32, check_tty=False).iter(range(32)):
    sleep(0.1)

print(f"Done! a * b = {a * b}")
