/**
 * NoirAudio - Zero-dependency Procedural Web Audio Engine for Detetive IA
 * Diegetic sound synthesis for mechanical typewriter, pocket watch, ink stamp, and paper rustle.
 */
(function (global) {
  'use strict';

  var AudioContextClass = global.AudioContext || global.webkitAudioContext;
  var ctx = null;
  var muted = false;
  var initialized = false;

  // Retrieve persistent mute preference
  try {
    var storedMute = localStorage.getItem('detiv_muted');
    if (storedMute !== null) {
      muted = storedMute === 'true';
    }
  } catch (e) {
    // LocalStorage might be unavailable in restricted environments
  }

  function getAudioContext() {
    if (!ctx && AudioContextClass) {
      ctx = new AudioContextClass();
    }
    if (ctx && ctx.state === 'suspended') {
      ctx.resume().catch(function () {});
    }
    return ctx;
  }

  var compressor = null;
  function getMasterNode(c) {
    if (!compressor && c) {
      try {
        compressor = c.createDynamicsCompressor();
        compressor.threshold.setValueAtTime(-18, c.currentTime);
        compressor.knee.setValueAtTime(12, c.currentTime);
        compressor.ratio.setValueAtTime(6, c.currentTime);
        compressor.attack.setValueAtTime(0.003, c.currentTime);
        compressor.release.setValueAtTime(0.25, c.currentTime);
        compressor.connect(c.destination);
      } catch (e) {
        return c.destination;
      }
    }
    return compressor || c.destination;
  }

  var rainSource = null;
  var rainGain = null;

  function updateToggleUI() {
    var btn = document.getElementById('sound-toggle');
    if (btn) {
      var icon = muted ? '#icon-mute' : '#icon-sound';
      var text = muted ? 'Som: Mudo' : 'Som: Ativo';
      btn.innerHTML = '<svg class="noir-icon" aria-hidden="true"><use href="' + icon + '"></use></svg> <span>' + text + '</span>';
      btn.setAttribute('aria-pressed', (!muted).toString());
      btn.title = muted ? 'Ativar efeitos sonoros diegéticos e chuva' : 'Desativar efeitos sonoros e chuva';
    }
  }

  var NoirAudio = {
    /**
     * Initializes user gesture listeners to unlock AudioContext
     */
    init: function () {
      if (initialized) {
        updateToggleUI();
        return;
      }
      initialized = true;

      var unlock = function () {
        var c = getAudioContext();
        if (c && c.state === 'running') {
          window.removeEventListener('click', unlock, true);
          window.removeEventListener('keydown', unlock, true);
          window.removeEventListener('touchstart', unlock, true);
          if (!muted) {
            NoirAudio.startRain();
          }
        }
      };

      window.addEventListener('click', unlock, true);
      window.addEventListener('keydown', unlock, true);
      window.addEventListener('touchstart', unlock, true);

      // Bind sound-toggle button if already present in DOM
      var btn = document.getElementById('sound-toggle');
      if (btn) {
        btn.onclick = function (e) {
          e.stopPropagation();
          NoirAudio.toggleMute();
        };
      }
      updateToggleUI();
    },

    /**
     * Toggles mute state and saves to localStorage
     * @returns {boolean} New mute state
     */
    toggleMute: function () {
      muted = !muted;
      try {
        localStorage.setItem('detiv_muted', muted.toString());
      } catch (e) {}
      if (muted) {
        NoirAudio.stopRain();
      } else {
        NoirAudio.startRain();
      }
      updateToggleUI();
      return muted;
    },

    /**
     * Returns true if sound is muted
     * @returns {boolean}
     */
    isMuted: function () {
      return muted;
    },

    /**
     * Mechanical typewriter key click (sharp noise transient + short metallic clack)
     */
    playTypewriter: function () {
      if (muted) return;
      var c = getAudioContext();
      if (!c) return;

      var t = c.currentTime;

      // 1. Noise burst transient for mechanical strike
      var bufferSize = Math.floor(c.sampleRate * 0.035);
      var buffer = c.createBuffer(1, bufferSize, c.sampleRate);
      var data = buffer.getChannelData(0);
      for (var i = 0; i < bufferSize; i++) {
        data[i] = (Math.random() * 2 - 1) * Math.exp(-i / (bufferSize * 0.2));
      }

      var noise = c.createBufferSource();
      noise.buffer = buffer;

      var filter = c.createBiquadFilter();
      filter.type = 'highpass';
      filter.frequency.setValueAtTime(1400, t);

      var noiseGain = c.createGain();
      noiseGain.gain.setValueAtTime(0.28, t);
      noiseGain.gain.exponentialRampToValueAtTime(0.001, t + 0.035);

      noise.connect(filter);
      filter.connect(noiseGain);
      noiseGain.connect(getMasterNode(c));
      noise.start(t);

      // 2. Metallic resonant ping
      var osc = c.createOscillator();
      osc.type = 'triangle';
      osc.frequency.setValueAtTime(850 + (Math.random() * 120), t);
      osc.frequency.exponentialRampToValueAtTime(320, t + 0.025);

      var oscGain = c.createGain();
      oscGain.gain.setValueAtTime(0.18, t);
      oscGain.gain.exponentialRampToValueAtTime(0.001, t + 0.025);

      osc.connect(oscGain);
      oscGain.connect(getMasterNode(c));
      osc.start(t);
      osc.stop(t + 0.03);
    },

    /**
     * Pocket watch wood/metal tick
     */
    playClockTick: function () {
      if (muted) return;
      var c = getAudioContext();
      if (!c) return;

      var t = c.currentTime;

      var osc = c.createOscillator();
      osc.type = 'sine';
      osc.frequency.setValueAtTime(1200, t);
      osc.frequency.exponentialRampToValueAtTime(450, t + 0.02);

      var filter = c.createBiquadFilter();
      filter.type = 'bandpass';
      filter.frequency.setValueAtTime(1100, t);
      filter.Q.setValueAtTime(5, t);

      var gain = c.createGain();
      gain.gain.setValueAtTime(0.22, t);
      gain.gain.exponentialRampToValueAtTime(0.0001, t + 0.02);

      osc.connect(filter);
      filter.connect(gain);
      gain.connect(getMasterNode(c));

      osc.start(t);
      osc.stop(t + 0.025);
    },

    /**
     * Solid rubber ink stamp hitting paper on wooden desk
     */
    playStamp: function () {
      if (muted) return;
      var c = getAudioContext();
      if (!c) return;

      var t = c.currentTime;

      // 1. Heavy resonant body thud
      var osc = c.createOscillator();
      osc.type = 'sine';
      osc.frequency.setValueAtTime(160, t);
      osc.frequency.exponentialRampToValueAtTime(45, t + 0.12);

      var oscGain = c.createGain();
      oscGain.gain.setValueAtTime(0.55, t);
      oscGain.gain.exponentialRampToValueAtTime(0.001, t + 0.14);

      osc.connect(oscGain);
      oscGain.connect(getMasterNode(c));
      osc.start(t);
      osc.stop(t + 0.15);

      // 2. Paper slap noise
      var bufferSize = Math.floor(c.sampleRate * 0.07);
      var buffer = c.createBuffer(1, bufferSize, c.sampleRate);
      var data = buffer.getChannelData(0);
      for (var i = 0; i < bufferSize; i++) {
        data[i] = (Math.random() * 2 - 1) * Math.exp(-i / (bufferSize * 0.18));
      }

      var noise = c.createBufferSource();
      noise.buffer = buffer;

      var noiseFilter = c.createBiquadFilter();
      noiseFilter.type = 'lowpass';
      noiseFilter.frequency.setValueAtTime(850, t);

      var noiseGain = c.createGain();
      noiseGain.gain.setValueAtTime(0.35, t);
      noiseGain.gain.exponentialRampToValueAtTime(0.001, t + 0.07);

      noise.connect(noiseFilter);
      noiseFilter.connect(noiseGain);
      noiseGain.connect(getMasterNode(c));
      noise.start(t);
    },

    /**
     * Paper rustle / page turn sound
     */
    playPaper: function () {
      if (muted) return;
      var c = getAudioContext();
      if (!c) return;

      var t = c.currentTime;
      var duration = 0.14;
      var bufferSize = Math.floor(c.sampleRate * duration);
      var buffer = c.createBuffer(1, bufferSize, c.sampleRate);
      var data = buffer.getChannelData(0);
      for (var i = 0; i < bufferSize; i++) {
        var progress = i / bufferSize;
        var env = Math.sin(progress * Math.PI) * (0.75 + 0.25 * Math.random());
        data[i] = (Math.random() * 2 - 1) * env;
      }

      var noise = c.createBufferSource();
      noise.buffer = buffer;

      var filter = c.createBiquadFilter();
      filter.type = 'bandpass';
      filter.frequency.setValueAtTime(1750, t);
      filter.frequency.linearRampToValueAtTime(800, t + duration);
      filter.Q.setValueAtTime(1.4, t);

      var gain = c.createGain();
      gain.gain.setValueAtTime(0.18, t);
      gain.gain.exponentialRampToValueAtTime(0.001, t + duration);

      noise.connect(filter);
      filter.connect(gain);
      gain.connect(getMasterNode(c));

      noise.start(t);
    },

    /**
     * Diegetic low-frequency double heartbeat thump under stress
     */
    playHeartbeat: function () {
      if (muted) return;
      var c = getAudioContext();
      if (!c) return;

      var t = c.currentTime;
      var dest = getMasterNode(c);

      // Thump 1 (Lub)
      var osc1 = c.createOscillator();
      osc1.type = 'sine';
      osc1.frequency.setValueAtTime(68, t);
      osc1.frequency.exponentialRampToValueAtTime(32, t + 0.08);

      var g1 = c.createGain();
      g1.gain.setValueAtTime(0.35, t);
      g1.gain.exponentialRampToValueAtTime(0.001, t + 0.09);

      osc1.connect(g1);
      g1.connect(dest);
      osc1.start(t);
      osc1.stop(t + 0.1);

      // Thump 2 (Dub)
      var osc2 = c.createOscillator();
      osc2.type = 'sine';
      osc2.frequency.setValueAtTime(56, t + 0.12);
      osc2.frequency.exponentialRampToValueAtTime(28, t + 0.22);

      var g2 = c.createGain();
      g2.gain.setValueAtTime(0.26, t + 0.12);
      g2.gain.exponentialRampToValueAtTime(0.001, t + 0.23);

      osc2.connect(g2);
      g2.connect(dest);
      osc2.start(t + 0.12);
      osc2.stop(t + 0.24);
    },

    /**
     * Continuous atmospheric diegetic rain against window pane
     */
    startRain: function () {
      if (muted || rainSource) return;
      var c = getAudioContext();
      if (!c) return;
      try {
        var bufferSize = Math.floor(c.sampleRate * 2.5);
        var buffer = c.createBuffer(1, bufferSize, c.sampleRate);
        var data = buffer.getChannelData(0);
        var lastOut = 0.0;
        for (var i = 0; i < bufferSize; i++) {
          var white = Math.random() * 2 - 1;
          data[i] = (lastOut + (0.02 * white)) / 1.02;
          lastOut = data[i];
          data[i] *= 3.0;
        }

        rainSource = c.createBufferSource();
        rainSource.buffer = buffer;
        rainSource.loop = true;

        var filter = c.createBiquadFilter();
        filter.type = 'lowpass';
        filter.frequency.setValueAtTime(850, c.currentTime);

        rainGain = c.createGain();
        rainGain.gain.setValueAtTime(0.045, c.currentTime);

        rainSource.connect(filter);
        filter.connect(rainGain);
        rainGain.connect(getMasterNode(c));
        rainSource.start();
      } catch (e) {}
    },

    /**
     * Stops ambient rain
     */
    stopRain: function () {
      if (rainSource) {
        try {
          rainSource.stop();
          rainSource.disconnect();
        } catch (e) {}
        rainSource = null;
        rainGain = null;
      }
    },

    /**
     * Metallic lens focus click for forensic loupe
     */
    playLensFocus: function () {
      if (muted) return;
      var c = getAudioContext();
      if (!c) return;
      var t = c.currentTime;
      var dest = getMasterNode(c);

      var osc = c.createOscillator();
      osc.type = 'sine';
      osc.frequency.setValueAtTime(2200, t);
      osc.frequency.exponentialRampToValueAtTime(1200, t + 0.03);

      var g = c.createGain();
      g.gain.setValueAtTime(0.18, t);
      g.gain.exponentialRampToValueAtTime(0.001, t + 0.035);

      osc.connect(g);
      g.connect(dest);
      osc.start(t);
      osc.stop(t + 0.04);
    },

    /**
     * Thumbtack / corkboard pin click when pinning clues or suspects
     */
    playPin: function () {
      if (muted) return;
      var c = getAudioContext();
      if (!c) return;
      var t = c.currentTime;
      var dest = getMasterNode(c);

      var osc = c.createOscillator();
      osc.type = 'triangle';
      osc.frequency.setValueAtTime(1400, t);
      osc.frequency.exponentialRampToValueAtTime(300, t + 0.02);

      var g = c.createGain();
      g.gain.setValueAtTime(0.25, t);
      g.gain.exponentialRampToValueAtTime(0.001, t + 0.025);

      osc.connect(g);
      g.connect(dest);
      osc.start(t);
      osc.stop(t + 0.03);
    }
  };

  // Auto-init on load
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', function () {
      NoirAudio.init();
    });
  } else {
    NoirAudio.init();
  }

  window.NoirAudio = NoirAudio;
})(window);
