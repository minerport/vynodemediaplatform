package com.vynode.media

import android.app.Application
import com.vynode.media.data.ClientDatabase

class VyNodeApplication : Application() {
    val database by lazy { ClientDatabase.open(this) }
}
