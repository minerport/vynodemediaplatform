# Home Screen

Home is composed by the server from typed rows rather than hard-coded React requests. Each user owns an independent ordered layout. Defaults are Continue Watching, Recently Added Movies, and Recently Added Shows.

Resolvers support Continue Watching, recently added Movies/Shows, Watchlist, Favorites, manual Collection, Smart Collection, and Playlist rows. Source-backed rows store a typed source ID. Consumer Home omits empty or disabled rows; settings retains them for editing. Rows are limited to 50 per user and 50 items each. Invalid source rows are removed safely.

This resolver boundary allows future recommendation, Live TV, music, provider-trending, or shared-list sources without rewriting Home.
