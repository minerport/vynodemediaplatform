"""Deterministic TMDb-compatible Phase 3 validation provider. Test/development only."""
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse
import json, struct, time, zlib

def chunk(kind, data): return struct.pack(">I",len(data))+kind+data+struct.pack(">I",zlib.crc32(kind+data)&0xffffffff)
PNG = b"\x89PNG\r\n\x1a\n"+chunk(b"IHDR",struct.pack(">IIBBBBB",2,3,8,2,0,0,0))+chunk(b"IDAT",zlib.compress((b"\x00"+b"\x20\x80\xc0"*2)*3))+chunk(b"IEND",b"")

MOVIES = {
    "603": {"id":603,"title":"The Matrix","original_title":"The Matrix","release_date":"1999-03-31","runtime":136,"overview":"A hacker discovers the world is a simulation.","tagline":"Welcome to the Real World.","status":"Released","original_language":"en","vote_average":8.2,"vote_count":25000,"poster_path":"/matrix-poster.png","backdrop_path":"/matrix-backdrop.png","genres":[{"id":28,"name":"Action"},{"id":878,"name":"Science Fiction"}],"production_companies":[{"id":79,"name":"Village Roadshow Pictures"}],"external_ids":{"imdb_id":"tt0133093"},"credits":{"cast":[{"id":6384,"name":"Keanu Reeves","character":"Neo","order":0}],"crew":[{"id":9340,"name":"Lana Wachowski","job":"Director","department":"Directing"},{"id":9340,"name":"Lana Wachowski","job":"Screenplay","department":"Writing"}]}},
    "604": {"id":604,"title":"The Matrix Reloaded","original_title":"The Matrix Reloaded","release_date":"2003-05-15","runtime":138,"overview":"The story continues.","status":"Released","original_language":"en","vote_average":7.0,"vote_count":10000,"poster_path":"/reloaded-poster.png","backdrop_path":"/reloaded-backdrop.png","genres":[{"id":28,"name":"Action"}],"production_companies":[{"id":79,"name":"Village Roadshow Pictures"}],"external_ids":{"imdb_id":"tt0234215"},"credits":{"cast":[],"crew":[]}}
}
MOVIES["1001"]={**MOVIES["603"],"id":1001,"title":"The Thing","original_title":"The Thing","release_date":"1982-06-25","overview":"An Antarctic research team faces a shape-shifting alien.","external_ids":{"imdb_id":"tt0084787"}}
MOVIES["1002"]={**MOVIES["603"],"id":1002,"title":"The Thing","original_title":"The Thing","release_date":"2011-10-14","overview":"A prequel set at the Norwegian camp.","external_ids":{"imdb_id":"tt0905372"}}
SHOW = {"id":10,"name":"Example Show","original_name":"Example Show","first_air_date":"2020-01-01","overview":"A deterministic example show.","status":"Returning Series","original_language":"en","vote_average":8.0,"vote_count":100,"poster_path":"/show-poster.png","backdrop_path":"/show-backdrop.png","genres":[{"id":18,"name":"Drama"}],"production_companies":[{"id":20,"name":"Example Studio"}],"created_by":[{"id":30,"name":"Example Creator"}],"external_ids":{"tvdb_id":1000},"credits":{"cast":[{"id":31,"name":"Example Actor","character":"Lead","order":0}],"crew":[]}}
SEASON = {"id":101,"season_number":1,"name":"Season 1","overview":"First season","air_date":"2020-01-01","episodes":[{"id":102,"episode_number":2,"name":"Second","overview":"Episode two","air_date":"2020-01-08","runtime":45,"still_path":"/episode-2.png"},{"id":103,"episode_number":3,"name":"Third","overview":"Episode three","air_date":"2020-01-15","runtime":46,"still_path":"/episode-3.png"}]}

class Handler(BaseHTTPRequestHandler):
    def log_message(self, *_): pass
    def send(self, value, status=200, mime="application/json"):
        body = value if isinstance(value, bytes) else json.dumps(value).encode()
        self.send_response(status); self.send_header("Content-Type", mime); self.send_header("Content-Length", str(len(body))); self.end_headers(); self.wfile.write(body)
    def do_GET(self):
        path=urlparse(self.path).path
        if path.startswith("/images/"): return self.send(PNG,200,"image/png")
        if self.headers.get("Authorization") != "Bearer phase3-test-token": return self.send({"status_message":"unauthorized"},401)
        if path=="/3/configuration": return self.send({"images":{"secure_base_url":"http://127.0.0.1:19090/images/"}})
        if path=="/3/search/movie":
            query=parse_qs(urlparse(self.path).query).get("query",[""])[0]
            if "Thing" in query: return self.send({"results":[{"id":1001,"title":"The Thing","release_date":"1982-06-25","overview":"1982 film"},{"id":1002,"title":"The Thing","release_date":"2011-10-14","overview":"2011 film"}]})
            return self.send({"results":[{"id":603,"title":"The Matrix","release_date":"1999-03-31","overview":"A hacker discovers the truth.","poster_path":"/matrix-poster.png","popularity":100}]})
        if path=="/3/search/tv": return self.send({"results":[{"id":10,"name":"Example Show","first_air_date":"2020-01-01","overview":"Example","poster_path":"/show-poster.png","popularity":50}]})
        if path.startswith("/3/movie/"): return self.send(MOVIES.get(path.rsplit("/",1)[-1],{}),200 if path.rsplit("/",1)[-1] in MOVIES else 404)
        if path=="/3/tv/10": return self.send(SHOW)
        if path=="/3/tv/10/season/1": return self.send(SEASON)
        if path=="/control/timeout": time.sleep(30); return
        return self.send({"status_message":"not found"},404)

if __name__ == "__main__": ThreadingHTTPServer(("0.0.0.0",19090),Handler).serve_forever()
