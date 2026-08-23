package com.vynode.media.data

import android.content.Context
import androidx.room.*
import kotlinx.coroutines.flow.Flow

@Entity(tableName = "servers", primaryKeys = ["serverId"])
data class ServerEntity(val serverId: String, val name: String, val endpoint: String, val insecureHttp: Boolean, val lastConnectedAt: Long?)

@Entity(tableName = "downloads", primaryKeys = ["serverId", "downloadId"])
data class DownloadEntity(val serverId: String, val downloadId: String, val logicalId: String, val title: String, val assetId: String, val sizeBytes: Long, val checksum: String, val localFile: String?, val state: String, val artworkUrl: String? = null, val localArtwork: String? = null)

@Entity(tableName = "sync_state", primaryKeys = ["serverId"])
data class SyncStateEntity(val serverId: String, val cursor: Long, val installationEpoch: String, val nextSequence: Long)

@Entity(tableName = "progress_queue", indices = [Index(value=["serverId", "eventId"], unique=true)])
data class ProgressEntity(@PrimaryKey(autoGenerate = true) val rowId: Long = 0, val serverId: String, val eventId: String, val sequence: Long, val logicalType: String, val logicalId: String, val positionSeconds: Double, val durationSeconds: Double, val occurredAt: String, val action: String?)

@Dao interface ClientDao {
    @Query("SELECT * FROM servers ORDER BY lastConnectedAt DESC") fun servers(): Flow<List<ServerEntity>>
    @Query("SELECT * FROM downloads WHERE serverId=:serverId ORDER BY title") fun downloads(serverId: String): Flow<List<DownloadEntity>>
    @Query("SELECT * FROM downloads WHERE serverId=:serverId AND state='READY' ORDER BY title") suspend fun readyDownloads(serverId: String): List<DownloadEntity>
    @Insert(onConflict = OnConflictStrategy.REPLACE) suspend fun saveServer(server: ServerEntity)
    @Insert(onConflict = OnConflictStrategy.REPLACE) suspend fun saveDownload(download: DownloadEntity)
    @Insert(onConflict = OnConflictStrategy.IGNORE) suspend fun enqueue(event: ProgressEntity)
    @Query("SELECT * FROM progress_queue WHERE serverId=:serverId ORDER BY sequence") suspend fun pendingProgress(serverId: String): List<ProgressEntity>
    @Query("DELETE FROM progress_queue WHERE serverId=:serverId AND eventId IN (:eventIds)") suspend fun removeProgress(serverId: String, eventIds: List<String>)
    @Query("SELECT * FROM sync_state WHERE serverId=:serverId") suspend fun syncState(serverId: String): SyncStateEntity?
    @Insert(onConflict = OnConflictStrategy.REPLACE) suspend fun saveSyncState(state: SyncStateEntity)
    @Query("SELECT * FROM servers ORDER BY lastConnectedAt DESC LIMIT 1") suspend fun lastServer(): ServerEntity?
    @Query("SELECT * FROM servers WHERE serverId=:serverId") suspend fun server(serverId: String): ServerEntity?
    @Query("SELECT * FROM downloads WHERE serverId=:serverId AND logicalId=:logicalId LIMIT 1") suspend fun downloadForMedia(serverId: String, logicalId: String): DownloadEntity?
}

@Database(entities=[ServerEntity::class, DownloadEntity::class, SyncStateEntity::class, ProgressEntity::class], version=2, exportSchema=true)
abstract class ClientDatabase : RoomDatabase() {
    abstract fun dao(): ClientDao
    companion object {
        private val MIGRATION_1_2 = object : androidx.room.migration.Migration(1, 2) {
            override fun migrate(db: androidx.sqlite.db.SupportSQLiteDatabase) {
                db.execSQL("ALTER TABLE downloads ADD COLUMN artworkUrl TEXT")
                db.execSQL("ALTER TABLE downloads ADD COLUMN localArtwork TEXT")
            }
        }
        fun open(context: Context) = Room.databaseBuilder(context, ClientDatabase::class.java, "vynode-client.db").addMigrations(MIGRATION_1_2).build()
    }
}
