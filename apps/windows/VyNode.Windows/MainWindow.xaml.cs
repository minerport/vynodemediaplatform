using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Windows.Graphics;

namespace VyNode.Windows;

public sealed partial class MainWindow : Window
{
    private readonly Auth.SecureCredentialStore _credentials = new();
    private readonly Services.LocalStateStore _state = new();
    private bool _started;
    public MainWindow()
    {
        InitializeComponent();
        ExtendsContentIntoTitleBar = true;
        SetTitleBar(TitleRegion);
        AppWindow.Title = "VyNode";
        AppWindow.Resize(new SizeInt32(1440, 900));
        if (AppWindow.Presenter is OverlappedPresenter presenter)
        {
            presenter.PreferredMinimumWidth = 900;
            presenter.PreferredMinimumHeight = 620;
        }
        Activated += MainWindow_Activated;
        ShowSignIn();
    }

    private async void MainWindow_Activated(object sender, WindowActivatedEventArgs args)
    {
        if (_started) return;
        _started = true;
        var cached = await _state.ReadAsync();
        if (cached is null) { ShowSignIn(); return; }
        var globalRefresh = _credentials.ReadGlobal(cached.User.Id);
        if (globalRefresh is not null)
        {
            try
            {
                var connect = new Services.ConnectClient();
                var login = await connect.RefreshAsync(globalRefresh, CancellationToken.None);
                _credentials.SaveGlobal(login.User.Id, login.RefreshToken);
                var servers = await connect.ServersAsync(login.AccessToken, CancellationToken.None);
                var selected = servers.FirstOrDefault(x => x.Id == cached.Server.Id) ?? servers.FirstOrDefault();
                if (selected is not null)
                {
                    ShowShell(await new Services.SessionBootstrapper().ConnectAsync(new Models.GlobalContext(login, servers, cached.Device), selected, CancellationToken.None));
                    return;
                }
                ShowZeroServer(new Models.GlobalContext(login, servers, cached.Device));
                return;
            }
            catch { /* Connect outage: continue with the identity-verified local session. */ }
        }
        try
        {
            var server = new Services.ServerClient();
            var identity = await server.IdentityAsync(cached.Endpoint, CancellationToken.None);
            if (!string.Equals(identity.InstanceId, cached.Server.Id, StringComparison.Ordinal)) throw new InvalidOperationException("Cached endpoint identity changed.");
            var refresh = _credentials.ReadServer(cached.User.Id, cached.Server.Id) ?? throw new InvalidOperationException("No cached server credential.");
            var local = await server.RefreshAsync(cached.Endpoint, refresh, CancellationToken.None);
            _credentials.SaveServer(cached.User.Id, cached.Server.Id, local.RefreshToken);
            ShowShell(new Models.SessionContext(cached.User, cached.Server, cached.Endpoint, local.AccessToken, null, cached.Servers, cached.Device, local.User?.Role));
        }
        catch { ShowSignIn(); }
    }

    public void ShowShell(Models.SessionContext session)
    {
        SetPlayerFullscreen(false);
        _ = _state.SaveAsync(session);
        ContentFrame.Navigate(typeof(ShellPage), session);
    }
    public void ShowPlayer(Models.PlaybackRoute route) => ContentFrame.Navigate(typeof(PlayerPage), route);
    public void ShowServerSelector(Models.GlobalContext context) => ContentFrame.Navigate(typeof(ServerSelectorPage), context);
    public void ShowZeroServer(Models.GlobalContext context) => ContentFrame.Navigate(typeof(ZeroServerPage), context);
    public void ShowSignIn() => ContentFrame.Navigate(typeof(SignInPage));
    public void ShowManualConnect(Models.GlobalContext? context = null) => ContentFrame.Navigate(typeof(ManualConnectPage), context);

    public void SetPlayerFullscreen(bool fullScreen)
    {
        TitleRegion.Visibility = fullScreen ? Visibility.Collapsed : Visibility.Visible;
        TitleRow.Height = new GridLength(fullScreen ? 0 : 48);
    }
}
