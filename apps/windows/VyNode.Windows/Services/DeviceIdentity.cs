namespace VyNode.Windows.Services;

public sealed class DeviceIdentity
{
    private readonly string _path = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData), "VyNode", "device.id");
    public string GetOrCreate()
    {
        if (File.Exists(_path) && Guid.TryParse(File.ReadAllText(_path).Trim(), out var saved)) return saved.ToString("D");
        Directory.CreateDirectory(Path.GetDirectoryName(_path)!);
        var id = Guid.NewGuid().ToString("D");
        File.WriteAllText(_path, id);
        File.SetAttributes(_path, FileAttributes.Hidden);
        return id;
    }
    public static string DisplayName => $"{Environment.UserName}'s Windows PC";
    public static Models.DeviceInput Describe() => new(DisplayName, "WINDOWS", "VyNode Desktop", "16.0.0", Environment.OSVersion.VersionString);
}
