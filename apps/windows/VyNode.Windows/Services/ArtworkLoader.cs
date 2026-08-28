using System.Collections.Concurrent;
using Microsoft.UI.Xaml.Media.Imaging;
using Windows.Storage.Streams;

namespace VyNode.Windows.Services;

public sealed class ArtworkLoader
{
    private readonly ServerClient _server;
    private readonly ConcurrentDictionary<string, Task<byte[]>> _cache = new();
    public ArtworkLoader(ServerClient server) => _server = server;

    public async Task<BitmapImage?> LoadAsync(string endpoint, string token, string? artworkId, CancellationToken ct)
    {
        if (string.IsNullOrWhiteSpace(artworkId)) return null;
        try
        {
            var key = $"{endpoint}|{artworkId}";
            var bytes = await _cache.GetOrAdd(key, _ => _server.ArtworkContentAsync(endpoint, token, artworkId, ct));
            using var stream = new InMemoryRandomAccessStream();
            using (var writer = new DataWriter(stream)) { writer.WriteBytes(bytes); await writer.StoreAsync(); }
            stream.Seek(0);
            var image = new BitmapImage();
            await image.SetSourceAsync(stream);
            return image;
        }
        catch { return null; }
    }

    public void Clear() => _cache.Clear();
}
