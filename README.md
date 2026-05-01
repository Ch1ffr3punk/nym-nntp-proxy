nym-nntp-proxy  
Use your favorite Usenet Newsreader and set  
the nntp server address to 127.0.0.1 and the  
port to 1199, to connect to the Nym Mixnet.  

You can define multiple nntp servers in the    
.json configuration file.  

Usage: nym-nntp-proxy [options]  

Options:    
-config string    
Path to config file (default: auto-search)   
-help  
Show usage Information   
-listen string   
Local port to listen on (e.g., 1199)   
-target string   
Target NNTP server (e.g., news.example.org:119) 

Examples:  
nym-nntp-proxy -listen 1199 -target news.example.org:119  
nym-nntp-proxy -config /path/to/proxy.json  
nym-nntp-proxy (uses default config file)  

The proxy always routes traffic through Nym Mixnet on 127.0.0.1:1080   
Make sure nym-socks5-client is running on port 1080  

But before you can use nym-nntp-proxy you will have    
to start the official [nym-socks5-client](https://github.com/nymtech/nym/releases/tag/nym-binaries-v2026.8-urda),   
with the following parameters:    

$ nym-socks5-client init --id 'your-nickname' --provider 'Nym address'     

$ nym-socks5-client run --id 'your-nickname' --use-anonymous-replies true,      

where 'Nym address' is the Nym address of a nym-network-requester,    
with whitelisted nntp server domains.   

I have a nym-network-requester running and you can obtain  
the required --provider credentials by joining [YIP](https://t.me/yourinternetprivacy).  

If you like nym-nntp-proxy consider a small donation in Crypto     
currencies or buy me a coffee.      
```  
BTC: bc1qkluy2kj8ay64jjsk0wrfynp8gvjwet9926rdel    
Nym: n1f0r6zzu5hgh4rprk2v2gqcyr0f5fr84zv69d3x    
XMR: 45TJx8ZHngM4GuNfYxRw7R7vRyFgfMVp862JqycMrPmyfTfJAYcQGEzT27wL1z5RG1b5XfRPJk97KeZr1svK8qES2z1uZrS      
```
