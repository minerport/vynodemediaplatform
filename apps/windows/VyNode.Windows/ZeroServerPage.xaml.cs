using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using VyNode.Windows.Models;
using VyNode.Windows.Services;
using VyNode.Windows.Auth;

namespace VyNode.Windows;

public sealed partial class ZeroServerPage : Page
{
    private GlobalContext _context = null!;
    private readonly ConnectClient _connect = new();
    private readonly UpdateApplicationService _updates = UpdateApplicationService.CreateDefault();
    public ZeroServerPage()
    {
        InitializeComponent();
        var currentVersion = typeof(App).Assembly.GetName().Version?.ToString(3) ?? "16.0.0";
        UpdateStatusText.Text = _updates.SafeMessage ?? $"VyNode {currentVersion} uses the {_updates.Trust?.Channel ?? "Stable"} update channel.";
        CheckUpdatesButton.IsEnabled = _updates.Trust is not null;
    }
    protected override void OnNavigatedTo(NavigationEventArgs e) => _context = (GlobalContext)e.Parameter;
    private async void Refresh_Click(object sender, RoutedEventArgs e)
    {
        try
        {
            var servers = await _connect.ServersAsync(_context.Login.AccessToken, CancellationToken.None);
            var next = _context with { Servers = servers };
            if (servers.Count == 0) { StatusText.Text = "No linked servers yet."; return; }
            ((MainWindow)App.Window).ShowServerSelector(next);
        }
        catch (Exception ex) { StatusText.Text = ex.Message; }
    }
    private void Manual_Click(object sender, RoutedEventArgs e) => ((MainWindow)App.Window).ShowManualConnect(_context);
    private async void CheckUpdates_Click(object sender, RoutedEventArgs e)
    {
        CheckUpdatesButton.IsEnabled = false;
        UpdateStatusText.Text = "Checking for updates…";
        var currentVersion = typeof(App).Assembly.GetName().Version?.ToString(3) ?? "16.0.0";
        try
        {
            var package = await _updates.CheckAndDownloadAsync(currentVersion, CancellationToken.None);
            UpdateStatusText.Text = _updates.SafeMessage ?? "Update check complete.";
            InstallUpdateButton.Visibility = package is null ? Visibility.Collapsed : Visibility.Visible;
        }
        catch (Exception ex) { UpdateStatusText.Text = ex.Message; }
        finally { CheckUpdatesButton.IsEnabled = _updates.Trust is not null; }
    }
    private async void InstallUpdate_Click(object sender, RoutedEventArgs e)
    {
        CheckUpdatesButton.IsEnabled = false;
        InstallUpdateButton.IsEnabled = false;
        UpdateStatusText.Text = "Launching Windows Installer…";
        try
        {
            var launched = await _updates.InstallReadyUpdateAsync(CancellationToken.None);
            UpdateStatusText.Text = _updates.SafeMessage ?? "Update installation finished.";
            RelaunchButton.Visibility = launched && _updates.State == UpdateRuntimeState.ReadyToRelaunch ? Visibility.Visible : Visibility.Collapsed;
            if (!launched) { CheckUpdatesButton.IsEnabled = true; InstallUpdateButton.IsEnabled = true; }
        }
        catch (Exception ex)
        {
            UpdateStatusText.Text = ex.Message;
            CheckUpdatesButton.IsEnabled = true;
            InstallUpdateButton.IsEnabled = true;
        }
    }
    private void Relaunch_Click(object sender, RoutedEventArgs e)
    {
        try { UpdateApplicationService.RelaunchInstalledApplication(); Application.Current.Exit(); }
        catch (Exception ex) { UpdateStatusText.Text = ex.Message; }
    }
    private async void SignOut_Click(object sender, RoutedEventArgs e)
    {
        try { await _connect.LogoutAsync(_context.Login.AccessToken, CancellationToken.None); } catch { }
        new SecureCredentialStore().ClearAccount(_context.Login.User.Id);
        new LocalStateStore().Clear();
        ((MainWindow)App.Window).ShowSignIn();
    }
}
