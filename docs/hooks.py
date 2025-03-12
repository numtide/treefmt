import os

def on_pre_build(**kwargs):
    os.system('nix run .#treefmt -- --help > ./snippets/usage.txt')