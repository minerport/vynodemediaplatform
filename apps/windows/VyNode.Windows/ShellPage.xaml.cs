using Microsoft.UI;
using Microsoft.UI.Text;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Automation;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Media.Imaging;
using Microsoft.UI.Xaml.Navigation;
using VirtualKey = global::Windows.System.VirtualKey;
using VyNode.Windows.Auth;
using VyNode.Windows.Models;
using VyNode.Windows.Services;

namespace VyNode.Windows;

public sealed partial class ShellPage : Page
{
    private SessionContext _session = null!;
    private readonly ServerClient _server = new();
    private readonly SessionBootstrapper _bootstrapper = new();
    private readonly SecureCredentialStore _credentials = new();
    private readonly LocalStateStore _state = new();
    private readonly ConnectClient _connect = new();
    private readonly ArtworkLoader _artwork;
    private readonly UpdateApplicationService _updates = UpdateApplicationService.CreateDefault();
    private CancellationTokenSource _pageCts = new();
    private Func<Task>? _backAction;
    private string _surface = "home";
    private string _lastSearchQuery = "";

    public ShellPage()
    {
        InitializeComponent();
        _artwork = new ArtworkLoader(_server);
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        _session = (SessionContext)e.Parameter;
        ServerButton.Content = _session.Server.Name;
        Navigation.SelectedItem = Navigation.MenuItems[0];
        await ShowHomeAsync();
    }

    private CancellationToken NextPage()
    {
        _pageCts.Cancel(); _pageCts.Dispose(); _pageCts = new CancellationTokenSource();
        _backAction = null; PageScroll.ChangeView(null, 0, null, true); ContentPanel.Children.Clear();
        return _pageCts.Token;
    }

    private async Task ShowHomeAsync()
    {
        _surface = "home"; var ct = NextPage(); AddPageHeading("Home", _session.Server.Name);
        try
        {
            var home = await _server.HomeAsync(_session.Endpoint, _session.ServerAccessToken, ct);
            var feature = home.Rows.SelectMany(x => x.Items).FirstOrDefault();
            if (feature is not null) ContentPanel.Children.Add(await FeatureAsync(feature, ct));
            foreach (var row in home.Rows) ContentPanel.Children.Add(await MediaRowAsync(row.Title, row.Items, ct));
            if (home.Rows.Count == 0) AddEmpty("Your Home is ready", "Add media to this server or adjust its Home rows in server settings.");
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { AddError(ex.Message, ShowHomeAsync); }
    }

    private async Task ShowMoviesAsync()
    {
        _surface = "movies"; var ct = NextPage(); AddPageHeading("Movies", "Your movie library");
        try
        {
            var movies = (await _server.MoviesAsync(_session.Endpoint, _session.ServerAccessToken, ct)).Movies;
            ContentPanel.Children.Add(MediaGrid(movies.Select(x => new CardData("MOVIE", x.Id, x.Title, x.Year > 0 ? x.Year.ToString() : "", null)).ToArray()));
            if (movies.Count == 0) AddEmpty("No movies available", "This server has no movies you can access.");
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { AddError(ex.Message, ShowMoviesAsync); }
    }

    private async Task ShowShowsAsync()
    {
        _surface = "shows"; var ct = NextPage(); AddPageHeading("Shows", "Your television library");
        try
        {
            var shows = (await _server.ShowsAsync(_session.Endpoint, _session.ServerAccessToken, ct)).Shows;
            ContentPanel.Children.Add(MediaGrid(shows.Select(x => new CardData("SHOW", x.Id, x.Title, x.Year > 0 ? x.Year.ToString() : "", null)).ToArray()));
            if (shows.Count == 0) AddEmpty("No shows available", "This server has no shows you can access.");
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { AddError(ex.Message, ShowShowsAsync); }
    }

    private void ShowSearchSurface(string query = "")
    {
        _surface = "search"; NextPage(); AddPageHeading("Search", "Find movies and shows on this server");
        var box = new AutoSuggestBox { PlaceholderText = "Search movies and shows", QueryIcon = new SymbolIcon(Symbol.Find), MaxWidth = 720, HorizontalAlignment = HorizontalAlignment.Stretch, Text = query };
        AutomationProperties.SetName(box, "Search movies and shows");
        box.QuerySubmitted += async (_, args) => await SearchAsync(args.QueryText);
        ContentPanel.Children.Add(box); box.Focus(FocusState.Programmatic);
        if (!string.IsNullOrWhiteSpace(query)) _ = SearchAsync(query);
    }

    private async Task SearchAsync(string query)
    {
        while (ContentPanel.Children.Count > 2) ContentPanel.Children.RemoveAt(2);
        if (string.IsNullOrWhiteSpace(query)) return;
        _lastSearchQuery = query.Trim();
        var progress = new ProgressRing { IsActive = true, Width = 28, HorizontalAlignment = HorizontalAlignment.Left };
        ContentPanel.Children.Add(progress);
        try
        {
            var result = await _server.SearchAsync(_session.Endpoint, _session.ServerAccessToken, query.Trim(), _pageCts.Token);
            ContentPanel.Children.Remove(progress);
            var movies = result.Movies ?? [];
            var shows = result.Shows ?? [];
            if (movies.Count == 0 && shows.Count == 0) { AddEmpty("No results", $"No media matched “{query.Trim()}”."); return; }
            if (movies.Count > 0) { ContentPanel.Children.Add(SectionHeading("Movies")); ContentPanel.Children.Add(MediaGrid(movies.Select(x => new CardData("MOVIE", x.Id, x.Title, x.Year.ToString(), null)).ToArray())); }
            if (shows.Count > 0) { ContentPanel.Children.Add(SectionHeading("Shows")); ContentPanel.Children.Add(MediaGrid(shows.Select(x => new CardData("SHOW", x.Id, x.Title, x.Year.ToString(), null)).ToArray())); }
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { if (ContentPanel.Children.Contains(progress)) ContentPanel.Children.Remove(progress); AddError(ex.Message, () => SearchAsync(query)); }
    }

    private async Task ShowMovieDetailAsync(string id)
    {
        var previous = _surface; _surface = "movie-detail"; var ct = NextPage();
        _backAction = previous == "search" ? () => { ShowSearchSurface(_lastSearchQuery); return Task.CompletedTask; } : previous == "home" ? ShowHomeAsync : ShowMoviesAsync;
        try
        {
            var movie = await _server.MovieAsync(_session.Endpoint, _session.ServerAccessToken, id, ct);
            var artwork = await _server.MovieArtworkAsync(_session.Endpoint, _session.ServerAccessToken, id, ct);
            var poster = artwork.FirstOrDefault(x => x.Selected && x.Type == "POSTER") ?? artwork.FirstOrDefault(x => x.Type == "POSTER");
            var backdrop = artwork.FirstOrDefault(x => x.Selected && x.Type == "BACKDROP") ?? artwork.FirstOrDefault(x => x.Type == "BACKDROP");
            ContentPanel.Children.Add(await DetailHeroAsync("MOVIE", movie.Id, movie.Title, movie.Overview, Metadata(movie.Year, movie.RuntimeMinutes, movie.Rating, movie.ContentRating, movie.Genres), poster?.Id, backdrop?.Id, ct));
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { AddError(ex.Message, () => ShowMovieDetailAsync(id)); }
    }

    private async Task ShowShowDetailAsync(string id)
    {
        var previous = _surface; _surface = "show-detail"; var ct = NextPage();
        _backAction = previous == "search" ? () => { ShowSearchSurface(_lastSearchQuery); return Task.CompletedTask; } : previous == "home" ? ShowHomeAsync : ShowShowsAsync;
        try
        {
            var show = await _server.ShowAsync(_session.Endpoint, _session.ServerAccessToken, id, ct);
            var artwork = await _server.ShowArtworkAsync(_session.Endpoint, _session.ServerAccessToken, id, ct);
            var poster = artwork.FirstOrDefault(x => x.Selected && x.Type == "POSTER") ?? artwork.FirstOrDefault(x => x.Type == "POSTER");
            var backdrop = artwork.FirstOrDefault(x => x.Selected && x.Type == "BACKDROP") ?? artwork.FirstOrDefault(x => x.Type == "BACKDROP");
            var seasons = show.Seasons ?? [];
            var firstEpisode = seasons.SelectMany(season => season.Episodes).FirstOrDefault(episode => episode.Available);
            ContentPanel.Children.Add(await DetailHeroAsync("SHOW", show.Id, show.Title, show.Overview, Metadata(show.Year, 0, show.Rating, null, show.Genres), poster?.Id, backdrop?.Id, ct, firstEpisode is null ? null : "EPISODE", firstEpisode?.Id, firstEpisode?.Title));
            if (seasons.Count == 0) { AddEmpty("No episodes available", "This show has no episodes you can access."); return; }
            var selector = new ComboBox { Header = "Season", MinWidth = 220, ItemsSource = seasons, DisplayMemberPath = "Title", SelectedIndex = 0 };
            ContentPanel.Children.Add(selector);
            var episodeHost = new StackPanel { Spacing = 12 }; ContentPanel.Children.Add(episodeHost);
            void Render(Season season)
            {
                episodeHost.Children.Clear();
                foreach (var episode in season.Episodes) episodeHost.Children.Add(EpisodeCard(season, episode));
            }
            selector.SelectionChanged += (_, _) => { if (selector.SelectedItem is Season season) Render(season); };
            Render(seasons[0]);
        }
        catch (OperationCanceledException) { }
        catch (Exception ex) { AddError(ex.Message, () => ShowShowDetailAsync(id)); }
    }

    private void ShowAccount()
    {
        _surface = "account"; NextPage(); AddPageHeading("Account", "VyNode Account and current server");
        ContentPanel.Children.Add(SectionHeading("VyNode Account"));
        ContentPanel.Children.Add(InfoBlock(_session.User.DisplayName ?? _session.User.Username, $"@{_session.User.Username}"));
        var signOut = new Button { Content = "Sign Out", HorizontalAlignment = HorizontalAlignment.Left }; signOut.Click += async (_, _) => await SignOutAsync(); ContentPanel.Children.Add(signOut);
        ContentPanel.Children.Add(SectionHeading("Current Server")); ContentPanel.Children.Add(InfoBlock(_session.Server.Name, _session.Server.Relationship));
        if ((_session.LinkedServers?.Count ?? 0) > 1)
        {
            var change = new Button { Content = "Switch Server", Style = (Style)Application.Current.Resources["PrimaryButton"], HorizontalAlignment = HorizontalAlignment.Left };
            change.Click += (_, _) => ServerButton_Click(change, new RoutedEventArgs()); ContentPanel.Children.Add(change);
        }
    }

    private void ShowSettings()
    {
        _surface = "settings"; NextPage(); AddPageHeading("Settings", "Consumer preferences for VyNode Desktop");
        ContentPanel.Children.Add(SectionHeading("Playback"));
        ContentPanel.Children.Add(InfoBlock("Automatic quality", "Playback quality will be negotiated with each server using the Windows capability profile."));
        ContentPanel.Children.Add(SectionHeading("Updates"));
        var currentVersion = typeof(App).Assembly.GetName().Version?.ToString(3) ?? "16.0.0";
        var updateStatus = new TextBlock
        {
            Text = _updates.SafeMessage ?? $"VyNode {currentVersion} uses the {_updates.Trust?.Channel ?? "Stable"} update channel.",
            TextWrapping = TextWrapping.Wrap,
            Foreground = (Brush)Application.Current.Resources["TextFillColorSecondaryBrush"]
        };
        AutomationProperties.SetName(updateStatus, "Update status");
        ContentPanel.Children.Add(updateStatus);
        var updateActions = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 10 };
        var check = new Button { Content = "Check for Updates", IsEnabled = _updates.Trust is not null };
        AutomationProperties.SetName(check, "Check for VyNode updates");
        var install = new Button { Content = "Install Update", Style = (Style)Application.Current.Resources["PrimaryButton"], Visibility = Visibility.Collapsed };
        AutomationProperties.SetName(install, "Install verified VyNode update");
        var later = new Button { Content = "Later", Visibility = Visibility.Collapsed };
        var relaunch = new Button { Content = "Relaunch VyNode", Style = (Style)Application.Current.Resources["PrimaryButton"], Visibility = Visibility.Collapsed };
        AutomationProperties.SetName(relaunch, "Relaunch updated VyNode");
        AutomationProperties.SetName(later, "Install update later");
        check.Click += async (_, _) =>
        {
            check.IsEnabled = false;
            updateStatus.Text = "Checking for updates…";
            VerifiedUpdatePackage? package;
            try { package = await _updates.CheckAndDownloadAsync(currentVersion, _pageCts.Token); }
            catch (OperationCanceledException) { return; }
            updateStatus.Text = _updates.SafeMessage ?? "Update check complete.";
            install.Visibility = package is null ? Visibility.Collapsed : Visibility.Visible;
            later.Visibility = install.Visibility;
            check.IsEnabled = _updates.Trust is not null;
        };
        install.Click += async (_, _) =>
        {
            check.IsEnabled = false; install.IsEnabled = false; later.IsEnabled = false;
            updateStatus.Text = "Launching Windows Installer…";
            bool launched;
            try { launched = await _updates.InstallReadyUpdateAsync(_pageCts.Token); }
            catch (OperationCanceledException) { return; }
            updateStatus.Text = _updates.SafeMessage ?? "Update installation finished.";
            relaunch.Visibility = launched && _updates.State == UpdateRuntimeState.ReadyToRelaunch ? Visibility.Visible : Visibility.Collapsed;
            if (!launched) { check.IsEnabled = true; install.IsEnabled = true; later.IsEnabled = true; }
        };
        relaunch.Click += (_, _) =>
        {
            try { UpdateApplicationService.RelaunchInstalledApplication(); Application.Current.Exit(); }
            catch (Exception exception) { updateStatus.Text = exception.Message; }
        };
        later.Click += (_, _) =>
        {
            install.Visibility = Visibility.Collapsed;
            later.Visibility = Visibility.Collapsed;
            updateStatus.Text = "Update postponed. You can check again later.";
        };
        updateActions.Children.Add(check); updateActions.Children.Add(install); updateActions.Children.Add(later); updateActions.Children.Add(relaunch);
        ContentPanel.Children.Add(updateActions);
        ContentPanel.Children.Add(SectionHeading("Advanced"));
        ContentPanel.Children.Add(InfoBlock("Connection diagnostics", $"Current server: {_session.Server.Name}\nConnection: verified local/server session"));
        var manual = new Button { Content = "Connect Manually", HorizontalAlignment = HorizontalAlignment.Left }; manual.Click += (_, _) => ((MainWindow)App.Window).ShowManualConnect(); ContentPanel.Children.Add(manual);
        if (_session.LocalRole is "OWNER" or "ADMIN")
        {
            var admin = new Button { Content = "Open Server Admin", HorizontalAlignment = HorizontalAlignment.Left };
            admin.Click += async (_, _) => await global::Windows.System.Launcher.LaunchUriAsync(new Uri(_session.Endpoint)); ContentPanel.Children.Add(admin);
        }
    }

    private async void Navigation_SelectionChanged(NavigationView sender, NavigationViewSelectionChangedEventArgs args)
    {
        if (args.SelectedItemContainer?.Tag is not string target) return;
        switch (target) { case "home": await ShowHomeAsync(); break; case "movies": await ShowMoviesAsync(); break; case "shows": await ShowShowsAsync(); break; case "search": ShowSearchSurface(); break; case "account": ShowAccount(); break; case "settings": ShowSettings(); break; }
    }

    private async void GlobalSearch_QuerySubmitted(AutoSuggestBox sender, AutoSuggestBoxQuerySubmittedEventArgs args) { Navigation.SelectedItem = Navigation.MenuItems[3]; ShowSearchSurface(args.QueryText); await Task.CompletedTask; }

    private async void ServerButton_Click(object sender, RoutedEventArgs e)
    {
        if (_session.GlobalAccessToken is null || _session.Device is null || (_session.LinkedServers?.Count ?? 0) < 2) return;
        var dialog = new ContentDialog { Title = "Switch server", XamlRoot = XamlRoot, CloseButtonText = "Cancel" };
        var list = new ListView { ItemsSource = _session.LinkedServers, DisplayMemberPath = "Name", IsItemClickEnabled = true, MinWidth = 360 };
        dialog.Content = list;
        list.ItemClick += async (_, args) =>
        {
            dialog.Hide();
            if (args.ClickedItem is not LinkedServer selected || selected.Id == _session.Server.Id) return;
            try
            {
                var login = new LoginResponse(_session.GlobalAccessToken, "", _session.User);
                _session = await _bootstrapper.ConnectAsync(new GlobalContext(login, _session.LinkedServers!, _session.Device), selected, CancellationToken.None);
                await _state.SaveAsync(_session); ServerButton.Content = _session.Server.Name; _artwork.Clear(); await ShowHomeAsync();
            }
            catch (Exception ex) { AddError(ex.Message, ShowHomeAsync); }
        };
        await dialog.ShowAsync();
    }

    private async Task SignOutAsync()
    {
        _pageCts.Cancel(); _artwork.Clear();
        if (_session.GlobalAccessToken is not null)
        {
            try { await _connect.LogoutAsync(_session.GlobalAccessToken, CancellationToken.None); }
            catch { }
        }
        _credentials.ClearAccount(_session.User.Id); _state.Clear(); ((MainWindow)App.Window).ShowSignIn();
    }

    private async Task<FrameworkElement> FeatureAsync(MediaItem item, CancellationToken ct)
    {
        var grid = new Grid { Height = 330 };
        var border = new Border { Background = Brush("VyRaised"), CornerRadius = new CornerRadius(18) }; grid.Children.Add(border);
        var image = new Image { Stretch = Stretch.UniformToFill, Opacity = .48 }; grid.Children.Add(image);
        var featureArtwork = item.ArtworkId ?? await FindBackdropAsync(item.Kind, item.Id, ct);
        image.Source = await _artwork.LoadAsync(_session.Endpoint, _session.ServerAccessToken, featureArtwork, ct);
        var shade = new Border { Background = new LinearGradientBrush { StartPoint = new global::Windows.Foundation.Point(0, .5), EndPoint = new global::Windows.Foundation.Point(1, .5), GradientStops = { new GradientStop { Color = ColorHelper.FromArgb(245, 9, 10, 12), Offset = 0 }, new GradientStop { Color = ColorHelper.FromArgb(10, 9, 10, 12), Offset = 1 } } } }; grid.Children.Add(shade);
        var stack = new StackPanel { Margin = new Thickness(42, 42, 42, 36), Spacing = 12, VerticalAlignment = VerticalAlignment.Bottom, MaxWidth = 620, HorizontalAlignment = HorizontalAlignment.Left };
        stack.Children.Add(new TextBlock { Text = item.Kind == "EPISODE" ? "CONTINUE WATCHING" : "FEATURED", Foreground = Brush("VyAccent"), FontWeight = FontWeights.Bold, CharacterSpacing = 160 });
        stack.Children.Add(new TextBlock { Text = item.Title, FontSize = 42, FontWeight = FontWeights.Bold, Foreground = Brush("VyText"), TextTrimming = TextTrimming.CharacterEllipsis });
        if (!string.IsNullOrWhiteSpace(item.Subtitle)) stack.Children.Add(new TextBlock { Text = item.Subtitle, Foreground = Brush("VyMuted"), FontSize = 17 });
        var play = new Button { Content = item.Position > 0 ? "Resume" : "Play", Style = (Style)Application.Current.Resources["PrimaryButton"], HorizontalAlignment = HorizontalAlignment.Left };
        play.Click += (_, _) => RoutePlayback(item.Kind, item.Id, item.Title); stack.Children.Add(play); grid.Children.Add(stack); return grid;
    }

    private async Task<FrameworkElement> MediaRowAsync(string title, IReadOnlyList<MediaItem> items, CancellationToken ct)
    {
        var root = new StackPanel { Spacing = 10 }; root.Children.Add(SectionHeading(title));
        var row = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 14 };
        foreach (var item in items) row.Children.Add(MediaCard(new CardData(item.Kind, item.Id, item.Title, item.Subtitle ?? (item.Year > 0 ? item.Year.ToString() : ""), item.ArtworkId)));
        root.Children.Add(new ScrollViewer { Content = row, HorizontalScrollMode = ScrollMode.Enabled, HorizontalScrollBarVisibility = ScrollBarVisibility.Hidden, VerticalScrollMode = ScrollMode.Disabled });
        await Task.CompletedTask; return root;
    }

    private GridView MediaGrid(IReadOnlyList<CardData> items)
    {
        var grid = new GridView { IsItemClickEnabled = false, SelectionMode = ListViewSelectionMode.None, HorizontalAlignment = HorizontalAlignment.Stretch };
        foreach (var item in items) grid.Items.Add(MediaCard(item)); return grid;
    }

    private Button MediaCard(CardData data)
    {
        var button = new Button { Width = 178, Height = 270, Padding = new Thickness(0), Margin = new Thickness(0, 0, 10, 14), Background = Brush("VySurface"), CornerRadius = new CornerRadius(12), HorizontalContentAlignment = HorizontalAlignment.Stretch, VerticalContentAlignment = VerticalAlignment.Stretch };
        ToolTipService.SetToolTip(button, data.Title); AutomationProperties.SetName(button, $"{data.Title}, {data.Kind.ToLowerInvariant()}");
        var grid = new Grid(); grid.RowDefinitions.Add(new RowDefinition { Height = new GridLength(1, GridUnitType.Star) }); grid.RowDefinitions.Add(new RowDefinition { Height = GridLength.Auto });
        var image = new Image { Stretch = Stretch.UniformToFill }; Grid.SetRow(image, 0); grid.Children.Add(image);
        var fallback = new TextBlock { Text = data.Kind == "MOVIE" ? "MOVIE" : "SHOW", Foreground = Brush("VyMuted"), HorizontalAlignment = HorizontalAlignment.Center, VerticalAlignment = VerticalAlignment.Center, CharacterSpacing = 140 }; Grid.SetRow(fallback, 0); grid.Children.Insert(0, fallback);
        var text = new StackPanel { Padding = new Thickness(12, 10, 12, 12), Spacing = 3 };
        text.Children.Add(new TextBlock { Text = data.Title, Foreground = Brush("VyText"), FontWeight = FontWeights.SemiBold, TextTrimming = TextTrimming.CharacterEllipsis, MaxLines = 1 });
        if (!string.IsNullOrWhiteSpace(data.Meta)) text.Children.Add(new TextBlock { Text = data.Meta, Foreground = Brush("VyMuted"), FontSize = 12, TextTrimming = TextTrimming.CharacterEllipsis }); Grid.SetRow(text, 1); grid.Children.Add(text); button.Content = grid;
        button.Loaded += async (_, _) =>
        {
            var artworkId = data.ArtworkId ?? await FindPosterAsync(data.Kind, data.Id, _pageCts.Token);
            image.Source = await _artwork.LoadAsync(_session.Endpoint, _session.ServerAccessToken, artworkId, _pageCts.Token);
        };
        button.Click += async (_, _) => { if (data.Kind == "MOVIE") await ShowMovieDetailAsync(data.Id); else if (data.Kind == "SHOW") await ShowShowDetailAsync(data.Id); else RoutePlayback(data.Kind, data.Id, data.Title); };
        return button;
    }

    private async Task<string?> FindPosterAsync(string kind, string id, CancellationToken ct)
    {
        try { var all = kind == "MOVIE" ? await _server.MovieArtworkAsync(_session.Endpoint, _session.ServerAccessToken, id, ct) : await _server.ShowArtworkAsync(_session.Endpoint, _session.ServerAccessToken, id, ct); return (all.FirstOrDefault(x => x.Selected && x.Type == "POSTER") ?? all.FirstOrDefault(x => x.Type == "POSTER"))?.Id; }
        catch { return null; }
    }

    private async Task<string?> FindBackdropAsync(string kind, string id, CancellationToken ct)
    {
        if (kind is not ("MOVIE" or "SHOW")) return null;
        try { var all = kind == "MOVIE" ? await _server.MovieArtworkAsync(_session.Endpoint, _session.ServerAccessToken, id, ct) : await _server.ShowArtworkAsync(_session.Endpoint, _session.ServerAccessToken, id, ct); return (all.FirstOrDefault(x => x.Selected && x.Type == "BACKDROP") ?? all.FirstOrDefault(x => x.Type == "BACKDROP"))?.Id; }
        catch { return null; }
    }

    private async Task<FrameworkElement> DetailHeroAsync(string kind, string id, string title, string overview, string metadata, string? posterId, string? backdropId, CancellationToken ct, string? playbackKind = null, string? playbackId = null, string? playbackTitle = null)
    {
        var root = new Grid { MinHeight = 470 }; var backdrop = new Image { Height = 450, Stretch = Stretch.UniformToFill, Opacity = .42, VerticalAlignment = VerticalAlignment.Top }; backdrop.Source = await _artwork.LoadAsync(_session.Endpoint, _session.ServerAccessToken, backdropId, ct); root.Children.Add(backdrop);
        root.Children.Add(new Border { Background = new LinearGradientBrush { StartPoint = new global::Windows.Foundation.Point(.5, 0), EndPoint = new global::Windows.Foundation.Point(.5, 1), GradientStops = { new GradientStop { Color = ColorHelper.FromArgb(10, 9, 10, 12), Offset = 0 }, new GradientStop { Color = ColorHelper.FromArgb(255, 9, 10, 12), Offset = 1 } } } });
        var layout = new Grid { Margin = new Thickness(28, 150, 28, 20) }; layout.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(190) }); layout.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        var poster = new Border { Width = 170, Height = 255, Background = Brush("VySurface"), CornerRadius = new CornerRadius(12), VerticalAlignment = VerticalAlignment.Bottom }; var posterImage = new Image { Stretch = Stretch.UniformToFill }; poster.Child = posterImage; posterImage.Source = await _artwork.LoadAsync(_session.Endpoint, _session.ServerAccessToken, posterId, ct); layout.Children.Add(poster);
        var copy = new StackPanel { Spacing = 12, VerticalAlignment = VerticalAlignment.Bottom, MaxWidth = 760, HorizontalAlignment = HorizontalAlignment.Left }; Grid.SetColumn(copy, 1);
        copy.Children.Add(new TextBlock { Text = title, FontSize = 40, FontWeight = FontWeights.Bold, Foreground = Brush("VyText"), TextWrapping = TextWrapping.Wrap });
        copy.Children.Add(new TextBlock { Text = metadata, Foreground = Brush("VyMuted"), FontSize = 16 });
        copy.Children.Add(new TextBlock { Text = overview, Foreground = Brush("VyText"), Opacity = .88, TextWrapping = TextWrapping.Wrap, MaxLines = 4, TextTrimming = TextTrimming.CharacterEllipsis, FontSize = 17 });
        var actions = new StackPanel { Orientation = Orientation.Horizontal, Spacing = 10 };
        var play = new Button { Content = "Play", Style = (Style)Application.Current.Resources["PrimaryButton"], HorizontalAlignment = HorizontalAlignment.Left };
        playbackKind ??= kind; playbackId ??= id; playbackTitle ??= title;
        var hasResume = false;
        if (playbackId is not null) { try { var progress = await _server.ProgressAsync(_session.Endpoint, _session.ServerAccessToken, playbackKind, playbackId, ct); hasResume = progress.Position > 0 && !progress.Watched; if (hasResume) play.Content = "Resume"; } catch { } }
        play.IsEnabled = playbackId is not null;
        play.Click += (_, _) => RoutePlayback(playbackKind, playbackId!, playbackTitle, hasResume); actions.Children.Add(play);
        if (hasResume) { var startOver = new Button { Content = "Start Over" }; startOver.Click += (_, _) => RoutePlayback(playbackKind, playbackId!, playbackTitle, false); actions.Children.Add(startOver); }
        copy.Children.Add(actions); layout.Children.Add(copy); root.Children.Add(layout); return root;
    }

    private Border EpisodeCard(Season season, Episode episode)
    {
        var border = new Border { Background = Brush("VySurface"), CornerRadius = new CornerRadius(12), Padding = new Thickness(16), MaxWidth = 980, HorizontalAlignment = HorizontalAlignment.Left };
        var button = new Button { Background = new SolidColorBrush(Colors.Transparent), HorizontalContentAlignment = HorizontalAlignment.Stretch, Padding = new Thickness(0), IsEnabled = episode.Available };
        AutomationProperties.SetName(button, $"S{season.SeasonNumber:00}E{episode.EpisodeNumber:00}, {episode.Title}, {(episode.RuntimeMinutes > 0 ? $"{episode.RuntimeMinutes} minutes" : "episode")}");
        var grid = new Grid { MinHeight = 110 }; grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(180) }); grid.ColumnDefinitions.Add(new ColumnDefinition { Width = new GridLength(1, GridUnitType.Star) });
        var art = new Border { Background = Brush("VyRaised"), CornerRadius = new CornerRadius(8), Margin = new Thickness(0, 0, 18, 0), Child = new TextBlock { Text = $"S{season.SeasonNumber:00}E{episode.EpisodeNumber:00}", Foreground = Brush("VyAccent"), HorizontalAlignment = HorizontalAlignment.Center, VerticalAlignment = VerticalAlignment.Center } }; grid.Children.Add(art);
        var text = new StackPanel { Spacing = 6, VerticalAlignment = VerticalAlignment.Center }; Grid.SetColumn(text, 1); text.Children.Add(new TextBlock { Text = episode.Title, FontSize = 19, FontWeight = FontWeights.SemiBold, Foreground = Brush("VyText") });
        text.Children.Add(new TextBlock { Text = episode.RuntimeMinutes > 0 ? $"{episode.RuntimeMinutes} min" : "Episode", Foreground = Brush("VyMuted") }); text.Children.Add(new TextBlock { Text = episode.Overview, Foreground = Brush("VyMuted"), TextWrapping = TextWrapping.Wrap, MaxLines = 2, TextTrimming = TextTrimming.CharacterEllipsis }); grid.Children.Add(text); button.Content = grid; button.Click += (_, _) => RoutePlayback("EPISODE", episode.Id, episode.Title); border.Child = button; return border;
    }

    private void RoutePlayback(string kind, string id, string title, bool resume = true) => App.Window.ShowPlayer(new PlaybackRoute(_session, kind, id, title, resume));

    private async void Page_KeyDown(object sender, KeyRoutedEventArgs e)
    {
        if (e.Key == VirtualKey.Escape && _backAction is not null) { await _backAction(); e.Handled = true; }
        if (e.Key == VirtualKey.K && Microsoft.UI.Input.InputKeyboardSource.GetKeyStateForCurrentThread(VirtualKey.Control).HasFlag(global::Windows.UI.Core.CoreVirtualKeyStates.Down)) { GlobalSearch.Focus(FocusState.Keyboard); e.Handled = true; }
    }
    private void ShowManualInfo() => ContentPanel.Children.Add(new InfoBar { IsOpen = true, Severity = InfoBarSeverity.Informational, Title = "Advanced manual connection", Message = "Manual local authentication and pairing is available when Connect cannot be reached." });
    private void AddPageHeading(string title, string subtitle) { ContentPanel.Children.Add(new StackPanel { Children = { new TextBlock { Text = title, FontSize = 34, FontWeight = FontWeights.Bold, Foreground = Brush("VyText") }, new TextBlock { Text = subtitle, Foreground = Brush("VyMuted") } } }); }
    private void AddEmpty(string title, string message) => ContentPanel.Children.Add(InfoBlock(title, message));
    private void AddError(string message, Func<Task> retry) { var bar = new InfoBar { IsOpen = true, Severity = InfoBarSeverity.Error, Title = "Unable to load this screen", Message = message, IsClosable = true }; var button = new Button { Content = "Retry" }; button.Click += async (_, _) => await retry(); bar.ActionButton = button; ContentPanel.Children.Add(bar); }
    private static TextBlock SectionHeading(string text) => new() { Text = text, FontSize = 22, FontWeight = FontWeights.SemiBold, Foreground = Brush("VyText"), Margin = new Thickness(0, 8, 0, 0) };
    private static Border InfoBlock(string title, string message) { var stack = new StackPanel { Spacing = 7 }; stack.Children.Add(new TextBlock { Text = title, FontSize = 20, FontWeight = FontWeights.SemiBold, Foreground = Brush("VyText") }); stack.Children.Add(new TextBlock { Text = message, Foreground = Brush("VyMuted"), TextWrapping = TextWrapping.Wrap }); return new Border { Background = Brush("VySurface"), CornerRadius = new CornerRadius(12), Padding = new Thickness(18), MaxWidth = 720, HorizontalAlignment = HorizontalAlignment.Left, Child = stack }; }
    private static string Metadata(int year, int runtime, double rating, string? contentRating, IReadOnlyList<string>? genres) => string.Join("  •  ", new[] { year > 0 ? year.ToString() : null, runtime > 0 ? $"{runtime} min" : null, rating > 0 ? $"★ {rating:0.0}" : null, contentRating, genres is { Count: > 0 } ? string.Join(", ", genres.Take(3)) : null }.Where(x => !string.IsNullOrWhiteSpace(x)));
    private static Brush Brush(string key) => (Brush)Application.Current.Resources[key];
    private sealed record CardData(string Kind, string Id, string Title, string Meta, string? ArtworkId);
}
