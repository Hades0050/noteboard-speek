package ru.vlad.noteboard

import android.content.res.Configuration
import android.os.Bundle
import android.util.Log
import android.view.View
import android.webkit.WebView
import androidx.activity.enableEdgeToEdge
import androidx.core.view.ViewCompat
import androidx.core.view.WindowInsetsCompat

class MainActivity : TauriActivity() {
  // Android 15+ рисует приложение под системными панелями, отключить это уже
  // нельзя, а WebView про панели не знает: env(safe-area-inset-*) там нули.
  // Слушатель вешается на decorView — он инсеты получает всегда, в отличие от
  // самого WebView, — а отступы кладутся на контейнер контента.
  override fun onCreate(savedInstanceState: Bundle?) {
    enableEdgeToEdge()
    super.onCreate(savedInstanceState)

    ViewCompat.setOnApplyWindowInsetsListener(window.decorView) { _, insets ->
      val bars = insets.getInsets(
        WindowInsetsCompat.Type.systemBars()
          or WindowInsetsCompat.Type.displayCutout()
          or WindowInsetsCompat.Type.ime(),
      )
      Log.i("noteboard", "insets ${bars.left} ${bars.top} ${bars.right} ${bars.bottom}")
      findViewById<View>(android.R.id.content)
        ?.setPadding(bars.left, bars.top, bars.right, bars.bottom)
      WindowInsetsCompat.CONSUMED
    }
    ViewCompat.requestApplyInsets(window.decorView)
  }

  // Полоса отступа — это фон самого WebView, и в тёмной теме она не должна
  // светиться белым.
  override fun onWebViewCreate(webView: WebView) {
    val night = resources.configuration.uiMode and
      Configuration.UI_MODE_NIGHT_MASK == Configuration.UI_MODE_NIGHT_YES
    webView.setBackgroundColor(if (night) 0xFF171717.toInt() else 0xFFFFFFFF.toInt())
    ViewCompat.requestApplyInsets(window.decorView)
  }
}
