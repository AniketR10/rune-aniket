def get(url):
    # Simulates a dependency module named "requests"; referenced from
    # main.py as requests.get(...).
    return Response(url)


def post(url, body):
    return Response(url)


class Response:
    def __init__(self, url):
        self.url = url
