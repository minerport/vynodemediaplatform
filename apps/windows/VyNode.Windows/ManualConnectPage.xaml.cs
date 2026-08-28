using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using VyNode.Windows.Auth;
using VyNode.Windows.Models;
using VyNode.Windows.Services;

namespace VyNode.Windows;

public sealed partial class ManualConnectPage : Page
{
    private readonly ServerClient _server = new();
    private readonly SecureCredentialStore _credentials = new();
    public ManualConnectPage() => InitializeComponent();

    private async void Connect_Click(object sender, RoutedEventArgs e)
    {
        ConnectButton.IsEnabled = false;
        StatusText.Visibility = Visibility.Collapsed;
        try
        {
            var endpoint = AddressBox.Text.Trim().TrimEnd('/');
            if (!Uri.TryCreate(endpoint, UriKind.Absolute, out var uri) || (uri.Scheme != Uri.UriSchemeHttp && uri.Scheme != Uri.UriSchemeHttps))
                throw new InvalidOperationException("Enter a valid HTTP or HTTPS server address.");
            var identity = await _server.IdentityAsync(endpoint, CancellationToken.None);
            var device = DeviceIdentity.Describe();
            var (session, user) = await _server.LoginAsync(endpoint, UsernameBox.Text.Trim(), PasswordBox.Password, device, CancellationToken.None);
            var accountId = $"local:{identity.InstanceId}:{user.Id}";
            var localUser = user with { Id = accountId };
            var linked = new LinkedServer(identity.InstanceId, identity.Name, "LOCAL", [new ServerEndpoint(endpoint, "local", null)]);
            _credentials.SaveServer(accountId, identity.InstanceId, session.RefreshToken);
            ((MainWindow)App.Window).ShowShell(new SessionContext(localUser, linked, endpoint, session.AccessToken, null, [linked], device, user.Role));
        }
        catch (Exception ex) { StatusText.Text = ex.Message; StatusText.Visibility = Visibility.Visible; }
        finally { ConnectButton.IsEnabled = true; }
    }

    private void Back_Click(object sender, RoutedEventArgs e) => ((MainWindow)App.Window).ShowSignIn();
}
