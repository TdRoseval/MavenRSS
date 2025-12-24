<template>
  <section class="hero-section">
    <canvas ref="canvasRef" class="particle-canvas" aria-hidden="true"></canvas>

    <div class="geometric-shapes">
      <div class="shape shape-1"></div>
      <div class="shape shape-2"></div>
      <div class="shape shape-3"></div>
    </div>

    <div class="hero-content">
      <h1 class="hero-title">
        <span class="title-line title-gradient">Read Smarter</span>
        <span class="title-line">with AI</span>
      </h1>

      <p class="hero-subtitle">Summarize, translate, and discover content intelligently</p>

      <div class="hero-actions">
        <button class="btn-primary" @mouseenter="onBtnHover" @mouseleave="onBtnLeave">
          <a class="btn-text" href="#download">Get MrRSS For Free</a>
          <PhArrowUpRight class="btn-icon" :size="20" weight="bold" />
        </button>
        <a href="#features" class="btn-secondary">Learn More</a>
      </div>

      <div class="hero-stats">
        <div class="stat-item">
          <span class="stat-number">100%</span>
          <span class="stat-label">Local Privacy</span>
        </div>
        <div class="stat-divider"></div>
        <div class="stat-item">
          <span class="stat-number">Free</span>
          <span class="stat-label">Forever</span>
        </div>
      </div>
    </div>

    <div class="scroll-indicator">
      <span class="scroll-text">Scroll to explore</span>
      <div class="scroll-line"></div>
    </div>
  </section>
</template>

<script setup>
import { ref, onMounted, onUnmounted } from 'vue';
import { PhArrowUpRight } from '@phosphor-icons/vue';

const canvasRef = ref(null);
let animationId = null;
let particles = [];
let mouseX = 0;
let mouseY = 0;

class Particle {
  constructor(canvas) {
    this.canvas = canvas;
    this.reset();
  }

  reset() {
    this.x = Math.random() * this.canvas.width;
    this.y = Math.random() * this.canvas.height;
    this.size = Math.random() * 3 + 1;
    this.speedX = (Math.random() - 0.5) * 0.5;
    this.speedY = (Math.random() - 0.5) * 0.5;
    this.opacity = Math.random() * 0.3 + 0.15;
    // Blue and yellow colors for light theme
    const colors = [210, 45, 200]; // Blue, golden yellow, mixed
    this.hue = colors[Math.floor(Math.random() * colors.length)];
  }

  update() {
    this.x += this.speedX;
    this.y += this.speedY;

    // Mouse interaction
    const dx = mouseX - this.x;
    const dy = mouseY - this.y;
    const distance = Math.sqrt(dx * dx + dy * dy);
    if (distance < 150) {
      const force = (150 - distance) / 150;
      this.x -= dx * force * 0.02;
      this.y -= dy * force * 0.02;
    }

    if (this.x < 0 || this.x > this.canvas.width) this.speedX *= -1;
    if (this.y < 0 || this.y > this.canvas.height) this.speedY *= -1;
  }

  draw(ctx) {
    ctx.beginPath();
    ctx.arc(this.x, this.y, this.size, 0, Math.PI * 2);
    // Lighter colors for light theme
    ctx.fillStyle = `hsla(${this.hue}, 80%, 45%, ${this.opacity})`;
    ctx.fill();
  }
}

const initParticles = () => {
  const canvas = canvasRef.value;
  if (!canvas) return;

  canvas.width = window.innerWidth;
  canvas.height = window.innerHeight;

  particles = [];
  const particleCount = Math.min(100, Math.floor((canvas.width * canvas.height) / 15000));
  for (let i = 0; i < particleCount; i++) {
    particles.push(new Particle(canvas));
  }
};

const animate = () => {
  const canvas = canvasRef.value;
  if (!canvas) return;

  const ctx = canvas.getContext('2d');
  ctx.clearRect(0, 0, canvas.width, canvas.height);

  // Draw connections
  particles.forEach((particle, i) => {
    particle.update();
    particle.draw(ctx);

    for (let j = i + 1; j < particles.length; j++) {
      const dx = particles[j].x - particle.x;
      const dy = particles[j].y - particle.y;
      const distance = Math.sqrt(dx * dx + dy * dy);

      if (distance < 120) {
        ctx.beginPath();
        ctx.strokeStyle = `rgba(0, 123, 255, ${0.12 * (1 - distance / 120)})`;
        ctx.lineWidth = 0.5;
        ctx.moveTo(particle.x, particle.y);
        ctx.lineTo(particles[j].x, particles[j].y);
        ctx.stroke();
      }
    }
  });

  animationId = requestAnimationFrame(animate);
};

const onBtnHover = (e) => {
  const btn = e.currentTarget;
  btn.style.transform = 'scale(1.05) translateY(-2px)';
};

const onBtnLeave = (e) => {
  const btn = e.currentTarget;
  btn.style.transform = 'scale(1) translateY(0)';
};

const handleMouseMove = (e) => {
  mouseX = e.clientX;
  mouseY = e.clientY;
};

const handleResize = () => {
  initParticles();
};

onMounted(() => {
  initParticles();
  animate();
  window.addEventListener('resize', handleResize);
  window.addEventListener('mousemove', handleMouseMove);
});

onUnmounted(() => {
  if (animationId) {
    cancelAnimationFrame(animationId);
  }
  window.removeEventListener('resize', handleResize);
  window.removeEventListener('mousemove', handleMouseMove);
});
</script>

<style scoped>
.hero-section {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4rem 2rem 2rem;
  overflow: hidden;
  background: linear-gradient(135deg, #f8f9fa 0%, #ffffff 50%, #f0f4f8 100%);
}

.particle-canvas {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
}

.geometric-shapes {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  pointer-events: none;
  overflow: hidden;
}

.shape {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.25;
  animation: float 20s ease-in-out infinite;
}

.shape-1 {
  width: 500px;
  height: 500px;
  background: linear-gradient(45deg, #007bff, #0056b3);
  top: -150px;
  right: -150px;
  animation-delay: 0s;
}

.shape-2 {
  width: 400px;
  height: 400px;
  background: linear-gradient(45deg, #ffc107, #ff9800);
  bottom: -100px;
  left: -100px;
  animation-delay: -5s;
}

.shape-3 {
  width: 350px;
  height: 350px;
  background: linear-gradient(45deg, #007bff, #ffc107);
  top: 40%;
  left: 15%;
  animation-delay: -10s;
}

@keyframes float {
  0%,
  100% {
    transform: translate(0, 0) rotate(0deg) scale(1);
  }
  25% {
    transform: translate(30px, -30px) rotate(90deg) scale(1.1);
  }
  50% {
    transform: translate(-20px, 20px) rotate(180deg) scale(0.9);
  }
  75% {
    transform: translate(20px, 30px) rotate(270deg) scale(1.05);
  }
}

.hero-content {
  position: relative;
  text-align: center;
  max-width: 1200px;
  z-index: 10;
}

@keyframes pulse {
  0%,
  100% {
    opacity: 1;
    transform: scale(1);
  }
  50% {
    opacity: 0.6;
    transform: scale(1.2);
  }
}

.hero-title {
  font-family: 'Space Grotesk', sans-serif;
  font-size: clamp(3.5rem, 14vw, 9rem);
  font-weight: 700;
  line-height: 0.85;
  margin-bottom: 2.5rem;
  text-transform: uppercase;
}

.title-line {
  display: block;
  opacity: 0;
  animation: slideInUp 0.8s ease-out forwards;
}

.title-line:nth-child(1) {
  animation-delay: 0.2s;
  transform: translateY(50px);
}

.title-line:nth-child(2) {
  animation-delay: 0.4s;
  transform: translateY(50px);
}

.title-gradient {
  background: linear-gradient(135deg, #007bff 0%, #ffc107 100%);
  -webkit-background-clip: text;
  -webkit-text-fill-color: transparent;
  background-clip: text;
  background-size: 200% 200%;
  animation:
    slideInUp 0.8s ease-out forwards,
    gradientShift 3s ease-in-out infinite;
}

@keyframes slideInUp {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes slideInDown {
  from {
    opacity: 0;
    transform: translateY(-20px);
  }
  to {
    opacity: 1;
    transform: translateY(0);
  }
}

@keyframes gradientShift {
  0%,
  100% {
    background-position: 0% 50%;
  }
  50% {
    background-position: 100% 50%;
  }
}

.hero-subtitle {
  font-size: clamp(1.125rem, 2.5vw, 1.5rem);
  color: rgba(0, 0, 0, 0.65);
  max-width: 650px;
  margin: 0 auto 3.5rem;
  line-height: 1.6;
  opacity: 0;
  animation: fadeIn 1s ease-out 0.8s forwards;
}

@keyframes fadeIn {
  to {
    opacity: 1;
  }
}

.hero-actions {
  display: flex;
  gap: 1rem;
  justify-content: center;
  flex-wrap: wrap;
  margin-bottom: 4.5rem;
  opacity: 0;
  animation: fadeIn 1s ease-out 1s forwards;
}

.btn-primary {
  display: inline-flex;
  align-items: center;
  gap: 0.75rem;
  padding: 1.125rem 2.5rem;
  background: linear-gradient(135deg, #007bff, #0056b3);
  border: none;
  border-radius: 12px;
  color: white;
  font-size: 1.125rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.3s ease;
  box-shadow: 0 10px 40px rgba(0, 123, 255, 0.25);
  font-family: 'Inter', sans-serif;
}

.btn-primary:hover {
  box-shadow: 0 15px 50px rgba(0, 123, 255, 0.35);
  transform: translateY(-2px);
}

.btn-secondary {
  display: inline-flex;
  align-items: center;
  padding: 1.125rem 2.5rem;
  background: transparent;
  border: 2px solid rgba(0, 0, 0, 0.12);
  border-radius: 12px;
  color: #1a1a1a;
  font-size: 1.125rem;
  font-weight: 600;
  cursor: pointer;
  text-decoration: none;
  transition: all 0.3s ease;
  font-family: 'Inter', sans-serif;
}

.btn-secondary:hover {
  background: rgba(0, 0, 0, 0.04);
  border-color: rgba(0, 0, 0, 0.2);
}

.hero-stats {
  display: flex;
  gap: 3rem;
  justify-content: center;
  flex-wrap: wrap;
  opacity: 0;
  animation: fadeIn 1s ease-out 1.2s forwards;
}

.stat-item {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.stat-number {
  font-family: 'Space Grotesk', sans-serif;
  font-size: 1.75rem;
  font-weight: 700;
  color: #007bff;
}

.stat-label {
  font-size: 0.875rem;
  color: rgba(0, 0, 0, 0.55);
  font-weight: 500;
}

.stat-divider {
  width: 1px;
  height: 40px;
  background: rgba(0, 0, 0, 0.1);
}

.scroll-indicator {
  position: absolute;
  bottom: 2rem;
  left: 50%;
  transform: translateX(-50%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  opacity: 0;
  animation: fadeIn 1s ease-out 1.5s forwards;
}

.scroll-text {
  font-size: 0.75rem;
  color: rgba(0, 0, 0, 0.45);
  text-transform: uppercase;
  letter-spacing: 2px;
  font-weight: 600;
}

.scroll-line {
  width: 1px;
  height: 40px;
  background: linear-gradient(to bottom, #007bff, transparent);
  animation: scrollLine 2s ease-in-out infinite;
}

@keyframes scrollLine {
  0%,
  100% {
    transform: scaleY(0);
    transform-origin: top;
  }
  50% {
    transform: scaleY(1);
    transform-origin: top;
  }
  51% {
    transform: scaleY(1);
    transform-origin: bottom;
  }
  100% {
    transform: scaleY(0);
    transform-origin: bottom;
  }
}

@media (max-width: 768px) {
  .hero-section {
    padding: 2rem 1rem 1rem;
    min-height: 100vh;
  }

  .shape {
    opacity: 0.15;
  }

  .shape-1 {
    width: 250px;
    height: 250px;
  }

  .shape-2 {
    width: 200px;
    height: 200px;
  }

  .shape-3 {
    width: 180px;
    height: 180px;
  }

  .hero-actions {
    flex-direction: column;
    width: 100%;
  }

  .btn-primary,
  .btn-secondary {
    width: 100%;
    justify-content: center;
  }

  .hero-stats {
    gap: 2rem;
  }

  .stat-divider {
    display: none;
  }
}

/* Handle small height screens */
@media (max-height: 980px) {
  .hero-section {
    padding: 2rem 2rem 1rem;
    min-height: 100vh;
  }

  .hero-title {
    margin-bottom: 1.5rem;
  }

  .hero-subtitle {
    margin-bottom: 2.5rem;
  }

  .hero-actions {
    margin-bottom: 3rem;
  }

  .hero-stats {
    margin-bottom: 3rem;
  }

  .scroll-indicator {
    bottom: 1rem;
  }

  .scroll-line {
    height: 30px;
  }
}

@media (max-width: 480px) and (max-height: 700px) {
  .hero-section {
    padding: 1.5rem 1rem 0.5rem;
  }

  .hero-title {
    margin-bottom: 1rem;
  }

  .hero-subtitle {
    margin-bottom: 2rem;
  }

  .hero-actions {
    margin-bottom: 2rem;
    gap: 0.75rem;
  }

  .btn-primary,
  .btn-secondary {
    padding: 0.875rem 1.5rem;
    font-size: 1rem;
  }
}
</style>
