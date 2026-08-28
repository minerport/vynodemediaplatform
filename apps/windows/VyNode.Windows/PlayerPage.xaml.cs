using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using Microsoft.UI.Xaml.Navigation;
using Windows.Media.Core;
using Windows.Media.Playback;
using Windows.System;
using Windows.System.Display;
using VyNode.Windows.Models;
using VyNode.Windows.Services;

namespace VyNode.Windows;

public sealed partial class PlayerPage : Page
{
    private readonly PlaybackClient _client = new();
    private readonly ServerClient _serverClient = new();
    private readonly MediaPlayer _player = new() { AutoPlay = false };
    private readonly DisplayRequest _displayRequest = new();
    private readonly DispatcherTimer _progressTimer = new() { Interval = TimeSpan.FromSeconds(1) };
    private readonly DispatcherTimer _syncTimer = new() { Interval = TimeSpan.FromSeconds(10) };
    private readonly DispatcherTimer _hideTimer = new() { Interval = TimeSpan.FromSeconds(4) };
    private readonly DispatcherTimer _upNextTimer = new() { Interval = TimeSpan.FromSeconds(1) };
    private PlaybackRoute _route = null!;
    private PlaybackSession? _serverSession;
    private CancellationTokenSource _lifetime = new();
    private bool _userWantsPlay = true;
    private bool _seeking;
    private bool _displayHeld;
    private bool _fullScreen;
    private bool _closing;
    private bool _menuOpen;
    private bool _subtitlesOff;
    private bool _upNextCanceled;
    private string? _qualityId;
    private string? _pendingAudioTrackId;
    private MediaPlaybackItem? _playbackItem;
    private IReadOnlyList<SubtitleCue> _subtitleCues = [];
    private int _upNextRemaining;

    public PlayerPage()
    {
        InitializeComponent();
        Video.SetMediaPlayer(_player);
        _player.MediaOpened += Player_MediaOpened;
        _player.MediaEnded += Player_MediaEnded;
        _player.MediaFailed += Player_MediaFailed;
        _player.PlaybackSession.PlaybackStateChanged += PlaybackSession_PlaybackStateChanged;
        _progressTimer.Tick += ProgressTimer_Tick;
        _syncTimer.Tick += async (_, _) => await SyncProgressAsync();
        _hideTimer.Tick += (_, _) => { _hideTimer.Stop(); if (_userWantsPlay && !AnyMenuOpen()) Controls.Visibility = Visibility.Collapsed; };
        _upNextTimer.Tick += UpNextTimer_Tick;
    }

    protected override async void OnNavigatedTo(NavigationEventArgs e)
    {
        _route = (PlaybackRoute)e.Parameter;
        TitleText.Text = _route.Title;
        await StartAsync(_route.LogicalType, _route.LogicalId, _route.Resume, _route.StartPosition);
    }

    private async Task StartAsync(string kind, string id, bool resume, double startPosition, string? audioId = null, string? subtitleId = null, string? qualityId = null, bool subtitlesOff = false)
    {
        await StopServerSessionAsync();
        _player.Source = null;
        _playbackItem = null;
        StateText.Text = "Starting";
        ErrorBar.IsOpen = false;
        var capabilities = WindowsCapabilityService.Current();
        if (subtitlesOff) capabilities = capabilities with { SubtitleFormats = [] };
        try
        {
            _serverSession = await _client.StartAsync(_route.Context, new PlaybackStartRequest(kind.ToUpperInvariant(), id, resume, capabilities, SelectedAudioTrackId: audioId, SelectedSubtitleTrackId: subtitleId, QualityId: qualityId, StartPosition: startPosition, ContextType: kind.Equals("EPISODE", StringComparison.OrdinalIgnoreCase) ? "TV_SERIES" : "MOVIE_SINGLE"), _lifetime.Token);
        }
        catch (Exception exception) when (exception is not OperationCanceledException)
        {
            StateText.Text = "Failed";
            ErrorBar.Message = $"Playback could not start. {exception.Message}";
            ErrorBar.IsOpen = true;
            ShowControls();
            return;
        }
        _route = _route with { LogicalType = kind, LogicalId = id };
        TitleText.Text = _route.Title;
        _subtitlesOff = subtitlesOff;
        _qualityId = qualityId;
        _pendingAudioTrackId = audioId ?? _serverSession.SelectedAudioTrack?.Id;
        var sourceUrl = _serverSession.HlsUrl ?? _serverSession.MediaUrl ?? throw new InvalidOperationException("The server did not provide a playable media URL.");
        var source = MediaSource.CreateFromUri(new Uri(sourceUrl));
        _subtitleCues = [];
        _upNextCanceled = false;
        SubtitleSurface.Visibility = Visibility.Collapsed;
        var item = new MediaPlaybackItem(source);
        _playbackItem = item;
        Video.Source = item;
        _player.Source = item;
        var start = startPosition > 0 ? startPosition : _serverSession.ResumePosition;
        _player.Play();
        if (start > 0) _player.PlaybackSession.Position = TimeSpan.FromSeconds(start);
        _userWantsPlay = true;
        if (!subtitlesOff && !string.IsNullOrWhiteSpace(_serverSession.SubtitleUrl))
        {
            try
            {
                var text = await _client.GetSubtitleAsync(_route.Context, _serverSession.SubtitleUrl, _lifetime.Token);
                _subtitleCues = WebVttParser.Parse(text);
            }
            catch (Exception exception) when (exception is not OperationCanceledException)
            {
                ErrorBar.Message = $"The selected subtitle track could not be loaded. {exception.Message}";
                ErrorBar.IsOpen = true;
            }
        }
        BuildMenus();
        ShowControls();
    }

    private void Player_MediaOpened(MediaPlayer sender, object args)
    {
        DispatcherQueue.TryEnqueue(() =>
        {
            var duration = _serverSession?.HlsUrl is not null ? _serverSession.Duration : sender.PlaybackSession.NaturalDuration.TotalSeconds;
            Timeline.Maximum = duration > 0 ? duration : Math.Max(1, _serverSession?.Duration ?? 1);
            DurationText.Text = Format(Timeline.Maximum);
            SelectPendingAudioTrack();
            _progressTimer.Start(); _syncTimer.Start(); StateText.Text = _serverSession?.Decision.Mode.Replace('_', ' ') ?? "Playing";
            HoldDisplay(true); ShowControls();
        });
    }

    private void SelectPendingAudioTrack()
    {
        if (_playbackItem is null || string.IsNullOrWhiteSpace(_pendingAudioTrackId) || _serverSession?.SelectedVersion.AudioTracks is not { Count: > 0 } tracks) return;
        var index = tracks.ToList().FindIndex(track => track.Id == _pendingAudioTrackId);
        if (index >= 0 && index < _playbackItem.AudioTracks.Count) _playbackItem.AudioTracks.SelectedIndex = index;
    }

    private void Player_MediaEnded(MediaPlayer sender, object args) => DispatcherQueue.TryEnqueue(async () => { await SyncProgressAsync("COMPLETED"); HoldDisplay(false); _progressTimer.Stop(); _syncTimer.Stop(); ShowControls(); if (!_upNextCanceled && _serverSession?.Navigation?.Next is not null) ShowUpNext(); });
    private void Player_MediaFailed(MediaPlayer sender, MediaPlayerFailedEventArgs args) => DispatcherQueue.TryEnqueue(() => { ErrorBar.Message = $"The media stream stopped unexpectedly ({args.Error}, 0x{args.ExtendedErrorCode.HResult:X8}). {args.ErrorMessage}"; ErrorBar.IsOpen = true; StateText.Text = "Failed"; HoldDisplay(false); ShowControls(); });

    private void PlaybackSession_PlaybackStateChanged(MediaPlaybackSession sender, object args) => DispatcherQueue.TryEnqueue(() =>
    {
        var playing = sender.PlaybackState == MediaPlaybackState.Playing;
        PlayPauseButton.Content = playing ? "Pause" : "Play";
        Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(PlayPauseButton, playing ? "Pause" : "Play");
        StateText.Text = sender.PlaybackState.ToString();
        HoldDisplay(playing);
    });

    private void ProgressTimer_Tick(object? sender, object e)
    {
        if (_seeking) return;
        var position = _player.PlaybackSession.Position.TotalSeconds;
        Timeline.Value = Math.Clamp(position, Timeline.Minimum, Timeline.Maximum);
        PositionText.Text = Format(position);
        var cue = _subtitleCues.FirstOrDefault(item => position >= item.Start && position < item.End);
        SubtitleText.Text = cue?.Text ?? string.Empty;
        SubtitleSurface.Visibility = cue is null ? Visibility.Collapsed : Visibility.Visible;
        var marker = _serverSession?.Markers?.FirstOrDefault(x => position >= x.Start && position < x.End && x.MarkerType is "INTRO" or "CREDITS");
        SkipButton.Visibility = marker is null ? Visibility.Collapsed : Visibility.Visible;
        if (marker is not null) { SkipButton.Content = marker.MarkerType == "INTRO" ? "Skip Intro" : "Skip Credits"; SkipButton.Tag = marker; Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(SkipButton, (string)SkipButton.Content); }
        var nav = _serverSession?.Navigation;
        if (!_upNextCanceled && nav?.Next is not null && nav.Autoplay && Timeline.Maximum > 0 && Timeline.Maximum - position <= Math.Max(5, nav.CountdownSeconds) && UpNextPanel.Visibility != Visibility.Visible) ShowUpNext();
    }

    private async Task SyncProgressAsync(string? forcedState = null)
    {
        if (_serverSession is null) return;
        var state = forcedState ?? (_userWantsPlay ? "PLAYING" : "PAUSED");
        var duration = Math.Max(Timeline.Maximum, _serverSession.Duration);
        var position = forcedState == "COMPLETED" ? duration : _player.PlaybackSession.Position.TotalSeconds;
        try { await _client.UpdateAsync(_route.Context, _serverSession.Id, new PlaybackProgressUpdate(state, position, duration), _lifetime.Token); }
        catch (OperationCanceledException) { }
        catch { /* transient progress failure must not interrupt playback */ }
    }

    private void PlayPause_Click(object sender, RoutedEventArgs e)
    {
        if (_userWantsPlay) { _userWantsPlay = false; _player.Pause(); _ = SyncProgressAsync("PAUSED"); }
        else { _userWantsPlay = true; _player.Play(); _ = SyncProgressAsync("PLAYING"); }
        ShowControls();
    }
    private void SeekBack_Click(object sender, RoutedEventArgs e) => SeekTo(_player.PlaybackSession.Position.TotalSeconds - 10);
    private void SeekForward_Click(object sender, RoutedEventArgs e) => SeekTo(_player.PlaybackSession.Position.TotalSeconds + 10);
    private void SeekTo(double seconds) { _player.PlaybackSession.Position = TimeSpan.FromSeconds(Math.Clamp(seconds, 0, Math.Max(0, Timeline.Maximum - .1))); _ = SyncProgressAsync(); ShowControls(); }
    private async void Retry_Click(object sender, RoutedEventArgs e)
    {
        var position = _player.PlaybackSession.Position.TotalSeconds;
        try
        {
            var identity = await _serverClient.IdentityAsync(_route.Context.Endpoint, _lifetime.Token);
            if (!string.Equals(identity.InstanceId, _route.Context.Server.Id, StringComparison.Ordinal))
                throw new InvalidOperationException("The endpoint returned a different server identity. No credential was sent.");
        }
        catch (Exception exception) when (exception is not OperationCanceledException)
        {
            StateText.Text = "Disconnected";
            ErrorBar.Message = $"The server is still unavailable. {exception.Message}";
            ErrorBar.IsOpen = true;
            ShowControls();
            return;
        }
        await StartAsync(_route.LogicalType, _route.LogicalId, false, position, _serverSession?.SelectedAudioTrack?.Id, _serverSession?.SelectedSubtitleTrack?.Id, _qualityId, _subtitlesOff);
    }
    private void Timeline_ValueChanged(object sender, Microsoft.UI.Xaml.Controls.Primitives.RangeBaseValueChangedEventArgs e) { if (_seeking) SeekTo(e.NewValue); }
    private void Timeline_PointerPressed(object sender, PointerRoutedEventArgs e) => _seeking = true;
    private void Timeline_PointerReleased(object sender, PointerRoutedEventArgs e) { _seeking = false; SeekTo(Timeline.Value); }

    private void BuildMenus()
    {
        AudioButton.IsEnabled = (_serverSession?.SelectedVersion.AudioTracks?.Count ?? 0) > 0;
        SubtitleButton.IsEnabled = (_serverSession?.SelectedVersion.SubtitleTracks?.Count ?? 0) > 0 || _serverSession?.SelectedSubtitleTrack is not null;
        QualityButton.IsEnabled = (_serverSession?.AvailableQualities?.Count ?? 0) > 0;
    }

    private void AudioButton_Click(object sender, RoutedEventArgs e) => ShowTrackMenu(AudioButton, _serverSession?.SelectedVersion.AudioTracks ?? [], _serverSession?.SelectedAudioTrack?.Id, false);
    private void SubtitleButton_Click(object sender, RoutedEventArgs e) => ShowTrackMenu(SubtitleButton, _serverSession?.SelectedVersion.SubtitleTracks ?? [], _serverSession?.SelectedSubtitleTrack?.Id, true);
    private void ShowTrackMenu(Button owner, IReadOnlyList<PlaybackTrack> tracks, string? selected, bool subtitles)
    {
        var menu = new MenuFlyout();
        _menuOpen = true;
        if (subtitles) { var off = new ToggleMenuFlyoutItem { Text = "Off", IsChecked = _subtitlesOff || selected is null }; off.Click += async (_, _) => await RestartAtCurrentAsync(subtitleId: "off"); menu.Items.Add(off); }
        foreach (var track in tracks.Where(x => x.Usable)) { var item = new ToggleMenuFlyoutItem { Text = TrackLabel(track), IsChecked = track.Id == selected }; item.Click += async (_, _) => { if (subtitles) await RestartAtCurrentAsync(subtitleId: track.Id); else await RestartAtCurrentAsync(audioId: track.Id); }; menu.Items.Add(item); }
        menu.Closed += (_, _) => { _menuOpen = false; owner.Focus(FocusState.Keyboard); ShowControls(); }; menu.ShowAt(owner);
    }

    private void QualityButton_Click(object sender, RoutedEventArgs e)
    {
        var menu = new MenuFlyout();
        _menuOpen = true;
        foreach (var quality in _serverSession?.AvailableQualities ?? []) { var item = new ToggleMenuFlyoutItem { Text = quality.Label, IsChecked = quality.Id == (_qualityId ?? (_serverSession?.Decision.Mode == "DIRECT_PLAY" ? "original" : null)) }; item.Click += async (_, _) => await RestartAtCurrentAsync(qualityId: quality.Id); menu.Items.Add(item); }
        menu.Closed += (_, _) => { _menuOpen = false; QualityButton.Focus(FocusState.Keyboard); ShowControls(); }; menu.ShowAt(QualityButton);
    }

    private async Task RestartAtCurrentAsync(string? audioId = null, string? subtitleId = null, string? qualityId = null)
    {
        var position = _player.PlaybackSession.Position.TotalSeconds;
        await StartAsync(_route.LogicalType, _route.LogicalId, false, position, audioId ?? _serverSession?.SelectedAudioTrack?.Id, subtitleId == "off" ? null : subtitleId ?? _serverSession?.SelectedSubtitleTrack?.Id, qualityId ?? _qualityId, subtitleId == "off");
    }

    private void SkipButton_Click(object sender, RoutedEventArgs e) { if (SkipButton.Tag is PlaybackMarker marker) SeekTo(marker.End); }
    private void ShowUpNext() { var nav = _serverSession?.Navigation; if (nav?.Next is null) return; _upNextRemaining = Math.Max(1, nav.CountdownSeconds); UpNextTitle.Text = nav.Next.Title; UpNextPanel.Visibility = Visibility.Visible; Controls.Visibility = Visibility.Visible; UpNextCountdown.Text = $"Playing in {_upNextRemaining} seconds"; _upNextTimer.Start(); }
    private void UpNextTimer_Tick(object? sender, object e) { _upNextRemaining--; UpNextCountdown.Text = $"Playing in {Math.Max(0, _upNextRemaining)} seconds"; if (_upNextRemaining <= 0) { _upNextTimer.Stop(); _ = PlayNextAsync(); } }
    private async void PlayNow_Click(object sender, RoutedEventArgs e) { _upNextTimer.Stop(); await PlayNextAsync(); }
    private void CancelUpNext_Click(object sender, RoutedEventArgs e) { _upNextCanceled = true; _upNextTimer.Stop(); UpNextPanel.Visibility = Visibility.Collapsed; ShowControls(); }
    private async Task PlayNextAsync() { var next = _serverSession?.Navigation?.Next; if (next is null) return; UpNextPanel.Visibility = Visibility.Collapsed; _route = _route with { LogicalType = "EPISODE", LogicalId = next.LogicalId, Title = next.Title, Resume = false, StartPosition = 0 }; await StartAsync("EPISODE", next.LogicalId, false, 0); }

    private void Fullscreen_Click(object sender, RoutedEventArgs e)
    {
        var window = App.Window;
        _fullScreen = !_fullScreen;
        window.SetPlayerFullscreen(_fullScreen);
        window.AppWindow.SetPresenter(_fullScreen ? AppWindowPresenterKind.FullScreen : AppWindowPresenterKind.Overlapped);
        FullscreenButton.Content = _fullScreen ? "Exit Fullscreen" : "Fullscreen";
        Microsoft.UI.Xaml.Automation.AutomationProperties.SetName(FullscreenButton, _fullScreen ? "Exit fullscreen" : "Enter fullscreen");
        ShowControls();
    }

    private void PlayerRoot_PointerMoved(object sender, PointerRoutedEventArgs e) => ShowControls();
    private void Video_PointerPressed(object sender, PointerRoutedEventArgs e) => ShowControls();
    private void ShowControls() { Controls.Visibility = Visibility.Visible; _hideTimer.Stop(); if (_userWantsPlay && !AnyMenuOpen()) _hideTimer.Start(); }
    private bool AnyMenuOpen() => _menuOpen || UpNextPanel.Visibility == Visibility.Visible;

    private async void CloseButton_Click(object sender, RoutedEventArgs e) => await CloseAsync();
    private async Task CloseAsync()
    {
        if (_closing) return; _closing = true;
        await SyncProgressAsync(); await StopServerSessionAsync(); HoldDisplay(false);
        if (_fullScreen)
        {
            App.Window.SetPlayerFullscreen(false);
            App.Window.AppWindow.SetPresenter(AppWindowPresenterKind.Overlapped);
        }
        App.Window.ShowShell(_route.Context);
    }

    private async Task StopServerSessionAsync() { var session = _serverSession; _serverSession = null; if (session is null) return; try { await _client.StopAsync(_route.Context, session.Id, CancellationToken.None); } catch { } }
    private void HoldDisplay(bool active) { if (active && !_displayHeld) { _displayRequest.RequestActive(); _displayHeld = true; } else if (!active && _displayHeld) { _displayRequest.RequestRelease(); _displayHeld = false; } }
    private static string Format(double seconds) => TimeSpan.FromSeconds(Math.Max(0, seconds)).ToString(seconds >= 3600 ? @"h\:mm\:ss" : @"m\:ss");
    private static string TrackLabel(PlaybackTrack track) => string.Join(" · ", new[] { track.Title, track.Language?.ToUpperInvariant(), track.Channels > 0 ? $"{track.Channels}ch" : null, track.Commentary ? "Commentary" : null }.Where(x => !string.IsNullOrWhiteSpace(x)));

    private async void Page_KeyDown(object sender, KeyRoutedEventArgs e)
    {
        ShowControls();
        if (e.Key == VirtualKey.Space) { PlayPause_Click(sender, e); e.Handled = true; }
        else if (e.Key == VirtualKey.Left) { SeekBack_Click(sender, e); e.Handled = true; }
        else if (e.Key == VirtualKey.Right) { SeekForward_Click(sender, e); e.Handled = true; }
        else if (e.Key == VirtualKey.Escape) { if (_fullScreen) Fullscreen_Click(sender, e); else await CloseAsync(); e.Handled = true; }
    }

    private async void Page_Unloaded(object sender, RoutedEventArgs e)
    {
        _progressTimer.Stop(); _syncTimer.Stop(); _hideTimer.Stop(); _upNextTimer.Stop(); HoldDisplay(false);
        if (_fullScreen) App.Window.SetPlayerFullscreen(false);
        if (!_closing) { await SyncProgressAsync(); await StopServerSessionAsync(); }
        _lifetime.Cancel(); _lifetime.Dispose(); _player.Dispose();
    }
}
