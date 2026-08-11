import os

import requests

from . import base


def fetch(name):
    base.check(name)
    return requests.get(os.environ.get("USER_URL", ""), timeout=1)
