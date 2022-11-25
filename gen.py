import hashlib
import os

# type
from io import TextIOWrapper

def main():
    l = []
    for path, d_list, f_list in os.walk('.'):
        print(path, d_list, f_list)
        if path == '.':
            for f_name in f_list:
                if f_name.endswith('.go') and f_name != 'main.go':
                    with open(os.path.join(path, f_name), 'rb') as f:
                        digest = file_digest(f)                    
                        print(digest[:8], f_name)
                        l.append((digest, f_name[:-3]))
            break
    s = print_main_go(l)
    with open('main.go', 'w') as f:
        f.write(s)
    os.system('export PATH=$PATH:/usr/local/go/bin;go build')
    # os.system('go build')
    # os.system('python -m http.server')

def file_digest(f:TextIOWrapper) -> str:
    m = hashlib.sha256()    
    m.update( f.read() )
    # print(m.hexdigest())
    return m.hexdigest()


def print_main_go(apis:list[tuple[str,str]]) -> str:
    txt = ''
    for path, fun_name in apis:
        txt += f'    http.HandleFunc("/{path[0:8]}/", {fun_name})\n'
        txt += f'    http.HandleFunc("/{fun_name}/", {fun_name})\n'
    return (f'''package main

import (
    "net/http"
)

func main() {{
    // auto generation start
{txt}    // end of auto generation
    
    http.ListenAndServe("127.111.111.111:8080", nil)
}}

''')

main()

