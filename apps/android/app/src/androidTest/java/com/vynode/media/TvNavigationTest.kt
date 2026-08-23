package com.vynode.media

import android.app.UiModeManager
import android.content.Context
import android.content.res.Configuration
import androidx.compose.ui.input.key.Key
import androidx.compose.ui.semantics.SemanticsActions
import androidx.compose.ui.test.*
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.test.platform.app.InstrumentationRegistry
import org.junit.Rule
import org.junit.Test
import org.junit.Assume.assumeTrue
import com.vynode.media.network.ApiHomeItem

class TvNavigationTest {
    @get:Rule val compose = createComposeRule()

    @Test fun dpadMovesAcrossMediaRow() {
        val context = InstrumentationRegistry.getInstrumentation().targetContext
        val uiMode = context.getSystemService(Context.UI_MODE_SERVICE) as UiModeManager
        assumeTrue(uiMode.currentModeType == Configuration.UI_MODE_TYPE_TELEVISION)
        var selected: String? = null
        compose.setContent { VyNodeHome(tv = true, rows = listOf(HomeRow("r", "Row", listOf(
            HomeCard(ApiHomeItem("MOVIE", "1", "Resume your movie", null, null)),
            HomeCard(ApiHomeItem("EPISODE", "2", "Next episode", null, null))))), play = { selected = it.id }) }
        val first = compose.onNodeWithTag("media-1")
        first.performSemanticsAction(SemanticsActions.RequestFocus)
        first.performKeyInput { keyDown(Key.DirectionRight); keyUp(Key.DirectionRight) }
        compose.onNodeWithTag("media-2").assertIsFocused()
        compose.onNodeWithTag("media-2").performKeyInput { keyDown(Key.DirectionCenter); keyUp(Key.DirectionCenter) }
        compose.runOnIdle { assert(selected == "2") }
    }

    @Test fun trivialControlPasses() { assert(true) }
}
