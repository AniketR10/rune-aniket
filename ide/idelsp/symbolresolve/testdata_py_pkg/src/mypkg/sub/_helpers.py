def slug(text):
    return text.lower().replace(" ", "-")


class Slugger:
    def run(self, text):
        return slug(text)