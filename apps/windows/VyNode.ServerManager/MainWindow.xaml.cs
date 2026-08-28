using System.Diagnostics;
using System.IO;
using System.Net.Http;
using System.Net.Http.Json;
using System.ServiceProcess;
using System.Windows;

namespace VyNode.ServerManager;

public partial class MainWindow : Window
{
    private const string ServiceName = "VyNodeMediaServer";
    private static readonly string InstallPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ProgramFiles), "VyNode", "Media Server");
    private static readonly string DataPath = Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.CommonApplicationData), "VyNode", "Media Server");
    private static readonly string LogsPath = Path.Combine(DataPath, "logs");

    public MainWindow() { InitializeComponent(); Loaded += async (_, _) => await RefreshStatusAsync(); }

    private async Task RefreshStatusAsync()
    {
        var executable = Path.Combine(InstallPath, "vynode-server.exe");
        var installed = File.Exists(executable);
        InstalledText.Text = installed ? "Installed" : "Not installed";
        VersionText.Text = installed ? FileVersionInfo.GetVersionInfo(executable).ProductVersion ?? "Unknown" : "—";
        PathText.Text = $"{DataPath}\n{LogsPath}";
        try
        {
            using var service = new ServiceController(ServiceName);
            ServiceText.Text = service.Status.ToString();
            StartButton.IsEnabled = service.Status == ServiceControllerStatus.Stopped;
            StopButton.IsEnabled = service.Status == ServiceControllerStatus.Running;
            RestartButton.IsEnabled = service.Status == ServiceControllerStatus.Running;
            if (service.Status == ServiceControllerStatus.Running)
            {
                try
                {
                    using var http = new HttpClient { Timeout = TimeSpan.FromSeconds(2) };
                    var version = await http.GetFromJsonAsync<VersionResponse>("http://127.0.0.1:8096/api/v1/system/version");
                    if (!string.IsNullOrWhiteSpace(version?.Version)) VersionText.Text = version.Version;
                }
                catch { }
            }
        }
        catch
        {
            ServiceText.Text = "Not installed";
            StartButton.IsEnabled = StopButton.IsEnabled = RestartButton.IsEnabled = false;
        }
    }

    private async Task ControlAsync(string action)
    {
        try
        {
            MessageText.Text = $"Requesting {action.ToLowerInvariant()}…";
            var process = Process.Start(new ProcessStartInfo("sc.exe", $"{action} \"{ServiceName}\"") { UseShellExecute = true, Verb = "runas", WindowStyle = ProcessWindowStyle.Hidden });
            if (process is null) throw new InvalidOperationException("Windows did not start Service Control Manager.");
            await process.WaitForExitAsync();
            if (process.ExitCode != 0) throw new InvalidOperationException($"Service Control Manager returned {process.ExitCode}.");
            var expected = action.Equals("start", StringComparison.OrdinalIgnoreCase) ? ServiceControllerStatus.Running : ServiceControllerStatus.Stopped;
            await Task.Run(() =>
            {
                using var service = new ServiceController(ServiceName);
                service.WaitForStatus(expected, TimeSpan.FromSeconds(20));
            });
            MessageText.Text = action.Equals("start", StringComparison.OrdinalIgnoreCase) ? "Server started." : action.Equals("stop", StringComparison.OrdinalIgnoreCase) ? "Server stopped." : "Service request completed.";
        }
        catch (Exception exception) { MessageText.Text = exception.Message; }
        await RefreshStatusAsync();
    }

    private async void Start_Click(object sender, RoutedEventArgs e) => await ControlAsync("start");
    private async void Stop_Click(object sender, RoutedEventArgs e) => await ControlAsync("stop");
    private async void Restart_Click(object sender, RoutedEventArgs e) { await ControlAsync("stop"); await ControlAsync("start"); }
    private async void Refresh_Click(object sender, RoutedEventArgs e) => await RefreshStatusAsync();
    private static void Open(string value) => Process.Start(new ProcessStartInfo(value) { UseShellExecute = true });
    private void OpenAdmin_Click(object sender, RoutedEventArgs e) => Open("http://127.0.0.1:8096/admin");
    private void OpenData_Click(object sender, RoutedEventArgs e) { Directory.CreateDirectory(DataPath); Open(DataPath); }
    private void OpenLogs_Click(object sender, RoutedEventArgs e) { Directory.CreateDirectory(LogsPath); Open(LogsPath); }

    private sealed record VersionResponse(string Version);
}
