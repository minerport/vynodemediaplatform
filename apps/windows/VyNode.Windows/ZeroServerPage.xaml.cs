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
    public ZeroServerPage() => InitializeComponent();
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
    private async void SignOut_Click(object sender, RoutedEventArgs e)
    {
        try { await _connect.LogoutAsync(_context.Login.AccessToken, CancellationToken.None); } catch { }
        new SecureCredentialStore().ClearAccount(_context.Login.User.Id);
        new LocalStateStore().Clear();
        ((MainWindow)App.Window).ShowSignIn();
    }
}
