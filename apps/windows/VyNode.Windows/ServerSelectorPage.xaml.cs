using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Navigation;
using VyNode.Windows.Models;
using VyNode.Windows.Services;

namespace VyNode.Windows;

public sealed partial class ServerSelectorPage : Page
{
    private GlobalContext _context = null!;
    private readonly SessionBootstrapper _bootstrapper = new();
    public ServerSelectorPage() => InitializeComponent();
    protected override void OnNavigatedTo(NavigationEventArgs e) { _context = (GlobalContext)e.Parameter; ServersList.ItemsSource = _context.Servers; }
    private async void Server_Click(object sender, ItemClickEventArgs e)
    {
        Busy.IsActive = true; ServersList.IsEnabled = false; ErrorText.Visibility = Visibility.Collapsed;
        try { ((MainWindow)App.Window).ShowShell(await _bootstrapper.ConnectAsync(_context, (LinkedServer)e.ClickedItem, CancellationToken.None)); }
        catch (Exception ex) { ErrorText.Text = ex.Message; ErrorText.Visibility = Visibility.Visible; }
        finally { Busy.IsActive = false; ServersList.IsEnabled = true; }
    }
}
