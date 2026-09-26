# MLB Postseason Pick'em

A small self-hosted app for you and your friends to pick winners (and MVPs)
for each round of the MLB postseason. Built to match your existing Pi stack:
Go backend, MySQL, Docker Compose, deployed behind your Nginx/Let's Encrypt/
DuckDNS setup the same way as Elite-Nine.

## How it works

- **Rounds**: seeded with ALDS x2, NLDS x2, ALCS, NLCS, and the World Series.
  Add more (e.g. Wild Card series) right from the Admin page's "Add a round"
  form - give it a unique key (like `ALWC1`), a best-of (Wild Card is best
  of 3), and whether it has an MVP (it doesn't).
- **Picks**: each player picks a team to win the series and the series
  length (e.g. "Dodgers in 5"). ALCS/NLCS/WS also require an MVP pick.
  Picks can be changed freely until the round's lock time.
- **All Picks board**: `/all-picks.html` shows everyone's picks for a round,
  but only once that round has locked - nothing leaks before the deadline.
- **Scoring** (as you specified):
  | Result | Points |
  |---|---|
  | Perfect (right team + right series length) | 1 |
  | Partial (right team, wrong length) | 0.5 |
  | Incorrect | 0 |
  | MVP bonus (ALCS/NLCS/WS only) | +2 |
- **MLB sync**: the admin page pulls Game 1's start time (to auto-set the
  lock) and the final winner/series length straight from MLB's public Stats
  API once you've set the matchup. MVP has to be entered manually - it isn't
  reliably available from that endpoint right when a series ends.

## Local setup

1. Copy `.env.example` to `.env` and fill in real passwords:
   ```
   cp .env.example .env
   ```
2. From the project root:
   ```
   docker compose up -d --build
   ```
3. On first boot the server creates an admin account using
   `ADMIN_USERNAME` / `ADMIN_PASSWORD` / `ADMIN_DISPLAY_NAME` from your
   `.env` (display name defaults to "Commissioner" if you leave it out).
   Log in with that account to reach `/admin.html`. This account also
   works as a normal player - it can submit picks just like anyone else,
   under the display name you set.

The `backend` service listens on container port 8080 (not published to the
host directly - see the Nginx section below).

## Deploying on your Pi (matching your Elite-Nine setup)

1. Copy this project's folder onto the Pi, next to your other projects.
2. Fill in `.env` there with real values (don't commit it).
3. `docker compose up -d --build` - this starts `pickem-db` (MySQL) and
   `pickem-app` (the Go server + static frontend) on an internal Docker
   network, exposing port 8080 only inside that network.
4. Add a path-based route in your existing Nginx config, the same pattern
   you're already using for Elite-Nine, something like:
   ```nginx
   location /pickem/ {
       proxy_pass http://127.0.0.1:PORT_YOU_MAP/;
       proxy_set_header Host $host;
       proxy_set_header X-Real-IP $remote_addr;
       proxy_set_header X-Forwarded-Proto $scheme;
   }
   ```
   You'll need to either publish `pickem-app`'s port 8080 to a free host
   port in `docker-compose.yml` (e.g. `ports: ["8091:8080"]`) so Nginx can
   reach it, or put Nginx on the same Docker network and proxy to
   `pickem-app:8080` directly - whichever matches how Elite-Nine is wired
   up. Since your cert already covers dklim.duckdns.org, this rides on the
   same HTTPS you've got.
5. If you'd rather give it its own subdomain instead of a path, that works
   too - just adjust the `server_name` / `location` blocks accordingly, and
   note the frontend uses root-relative paths (`/style.css`, `/app.js`,
   `/api/...`) so a path prefix like `/pickem/` will need either a
   dedicated server block or rewriting those paths - a subdomain is the
   easier option if you have one to spare.

## Admin workflow each round

1. Once a matchup is set (e.g. after the Wild Card round ends and the ALDS
   pairings are known), go to `/admin.html`, pick Team A / Team B from the
   dropdowns (pulled live from MLB's team list), and save.
2. Set the lock time either by clicking **"Sync Game 1 lock time from MLB"**
   with a date range covering the start of that round, or by entering it
   yourself under "Or set lock time manually" and clicking **Save lock
   time**. "Clear lock" reopens picks if you ever need to walk one back.
3. After the series ends, click **"Sync result from MLB"** to pull the
   winner and series length automatically.
4. Manually fill in the **Series MVP** field (and confirm winner/length if
   the auto-sync ever comes up short) and click **"Save result manually"**
   to mark the round Final - this is what makes it count on the
   leaderboard.

## Notes / things worth knowing

- Passwords are hashed with bcrypt; sessions are opaque tokens in a
  `sessions` table with a 30-day expiry cookie (`HttpOnly`, `Secure` once
  served over HTTPS - already on via `COOKIE_SECURE=true` in compose).
- The MLB Stats API (`statsapi.mlb.com`) is free and needs no API key, but
  it's an unofficial-for-third-parties public endpoint - if MLB ever
  changes its shape, the sync buttons might need small tweaks; the manual
  override on the admin page always works as a fallback.
- Dependencies are pinned in `go.mod`/`go.sum` (already generated and
  verified to build) - the Docker build just needs normal outbound internet
  access to fetch them, same as any other Go project.
- This was built as a first working pass. Nice-to-haves left out on purpose
  to keep it simple to stand up: email/password reset, push notifications
  before lock time, and a "pick summary" reveal page after a round locks
  (currently everyone can see who picked what once results are in, via the
  round card, but not before locking).
