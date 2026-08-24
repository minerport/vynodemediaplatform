package com.vynode.media

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.focusable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsFocusedAsState
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Shapes
import androidx.compose.material3.Surface
import androidx.compose.material3.Typography
import androidx.compose.material3.darkColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

object VyNodeColor {
    val Background = Color(0xFF090A0D)
    val BackgroundSubtle = Color(0xFF0F1014)
    val BackgroundRaised = Color(0xFF13151A)
    val Surface = Color(0xFF16181D)
    val Raised = Color(0xFF1D2026)
    val Hover = Color(0xFF252830)
    val Selected = Color(0xFF292B31)
    val Overlay = Color(0xF024272E)
    val Border = Color(0xFF2D3037)
    val Text = Color(0xFFF5F5F6)
    val Muted = Color(0xFFA9ABB2)
    val Subtle = Color(0xFF737680)
    val Disabled = Color(0xFF565962)
    val Accent = Color(0xFFF4A340)
    val AccentStrong = Color(0xFFFFC06A)
    val AccentInk = Color(0xFF241306)
    val Danger = Color(0xFFFF746C)
    val Warning = Color(0xFFE9BD68)
    val Info = Color(0xFF82AAFF)
    val Focus = Color(0xFFFFD08A)
}

object VyNodeSpace {
    val xs = 4.dp; val sm = 8.dp; val md = 12.dp; val lg = 16.dp
    val xl = 20.dp; val xxl = 24.dp; val xxxl = 32.dp; val display = 48.dp
}

object VyNodeRadius {
    val small = RoundedCornerShape(8.dp)
    val medium = RoundedCornerShape(12.dp)
    val large = RoundedCornerShape(18.dp)
    val pill = RoundedCornerShape(999.dp)
}

@Composable
fun VyNodeTheme(content: @Composable () -> Unit) = MaterialTheme(
    colorScheme = darkColorScheme(
        primary = VyNodeColor.Accent,
        onPrimary = VyNodeColor.AccentInk,
        secondary = VyNodeColor.AccentStrong,
        background = VyNodeColor.Background,
        surface = VyNodeColor.Surface,
        surfaceVariant = VyNodeColor.Raised,
        outline = VyNodeColor.Border,
        onBackground = VyNodeColor.Text,
        onSurface = VyNodeColor.Text,
        onSurfaceVariant = VyNodeColor.Muted,
        error = VyNodeColor.Danger,
    ),
    shapes = Shapes(small = VyNodeRadius.small, medium = VyNodeRadius.medium, large = VyNodeRadius.large),
    typography = Typography(
        displayLarge = Typography().displayLarge.copy(fontSize = 58.sp, lineHeight = 62.sp),
        headlineLarge = Typography().headlineLarge.copy(fontSize = 34.sp, lineHeight = 40.sp),
        headlineMedium = Typography().headlineMedium.copy(fontSize = 28.sp, lineHeight = 34.sp),
        titleLarge = Typography().titleLarge.copy(fontSize = 22.sp, lineHeight = 28.sp),
        titleMedium = Typography().titleMedium.copy(fontSize = 17.sp, lineHeight = 22.sp),
        bodyLarge = Typography().bodyLarge.copy(fontSize = 16.sp, lineHeight = 24.sp),
        bodyMedium = Typography().bodyMedium.copy(fontSize = 14.sp, lineHeight = 21.sp),
        labelLarge = Typography().labelLarge.copy(fontSize = 14.sp),
    ),
    content = {
        Surface(color = VyNodeColor.Background, contentColor = VyNodeColor.Text) {
            content()
        }
    },
)

@Composable
fun VyNodePrimaryButton(label: String, modifier: Modifier = Modifier, enabled: Boolean = true, onClick: () -> Unit) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    Button(
        onClick = onClick,
        enabled = enabled,
        interactionSource = interaction,
        modifier = modifier.then(if (focused) Modifier.border(3.dp, VyNodeColor.Focus, VyNodeRadius.pill) else Modifier),
        shape = VyNodeRadius.pill,
        contentPadding = PaddingValues(horizontal = 24.dp, vertical = 13.dp),
        colors = ButtonDefaults.buttonColors(containerColor = if (focused) VyNodeColor.AccentStrong else VyNodeColor.Accent, contentColor = VyNodeColor.AccentInk),
    ) { androidx.compose.material3.Text(label) }
}

@Composable
fun VyNodeSecondaryButton(label: String, modifier: Modifier = Modifier, enabled: Boolean = true, onClick: () -> Unit) {
    val interaction = remember { MutableInteractionSource() }
    val focused by interaction.collectIsFocusedAsState()
    OutlinedButton(
        onClick = onClick,
        enabled = enabled,
        interactionSource = interaction,
        modifier = modifier.then(if (focused) Modifier.border(3.dp, VyNodeColor.Focus, VyNodeRadius.pill) else Modifier),
        shape = VyNodeRadius.pill,
        border = BorderStroke(1.dp, if (focused) VyNodeColor.Focus else VyNodeColor.Border),
        contentPadding = PaddingValues(horizontal = 20.dp, vertical = 12.dp),
        colors = ButtonDefaults.outlinedButtonColors(contentColor = VyNodeColor.Text),
    ) { androidx.compose.material3.Text(label) }
}
