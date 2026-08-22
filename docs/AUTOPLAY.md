# Autoplay and playback context

Episode sessions carry a per-user playback context separate from authentication. The server resolves previous and next available standard episodes by aired season/episode order, skips unavailable media, crosses normal season boundaries, and excludes Specials from automatic progression.

At episode completion the player shows Up Next. With autoplay enabled it uses a cancellable 10-second countdown; otherwise Play Now remains manual. The next episode creates a new playback session and independently reapplies semantic track and quality preferences. Leaving through the player Back control ends the viewing context.
