import json
import os

def main():
    l = []
    imps = []
    for path, d_list, f_list in os.walk('.'):
        print(path, d_list, f_list)
        if path == '.':
            for d_name in d_list:
                if d_name.startswith('.'): continue                
                print(d_name)
                imps.append(f'api-pack/{d_name}')
                with open(os.path.join(path, d_name, 'router.go'), 'r') as f:
                    d = find_router(f)   
                    for k in d:           
                        if k.startswith('@'):
                            l.append((f'/{k[1:]}/', f'{d_name}.{d[k]}'))
                        else:                            
                            l.append((f'/{d_name}{k}', f'{d_name}.{d[k]}'))
            break
    s = print_main_go(imps, l)
    with open('main.go', 'w') as f:
        f.write(s)
    os.system('export PATH=$PATH:/usr/local/go/bin;go build')
    # os.system('go build')
    # os.system('python -m http.server')

def find_router(f):
    l = f.readline()
    txt = ''
    while l:
        l = f.readline()
        if len(l) > 3 and l.startswith('//'):
            txt += l[2:]
        
    d = json.loads(txt)
    return d

def print_main_go(imports,l):
    txt = ''
    for path, func in l:
        txt += f'r.HandleFunc("{path}",{func})\n'
    imps = ''
    for i in imports:
        imps += f'"{i}"\n'
    return f'''package main

import (
{imps}
	"net/http"

	"github.com/gorilla/mux"
)

func main() {{
	r := mux.NewRouter()
	// auto generate
{txt}
	// end auto generate
	http.ListenAndServe("127.111.111.111:8080", r)
}}

'''


main()
