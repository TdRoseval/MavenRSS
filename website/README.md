# MavenRSS Landing Page

A modern, visually stunning landing page for MavenRSS - an AI-powered RSS reader application.

![MavenRSS Landing Page](https://img.shields.io/badge/Vue.js-3.4+-42b883?style=flat&logo=vue.js&logoColor=white)
![Vite](https://img.shields.io/badge/Vite-5.0-646cff?style=flat&logo=vite&logoColor=white)

## Features

- **Modernist Design**: Bold visual elements with vibrant colors and high contrast
- **Animated Hero Section**: Interactive particle system with mouse tracking
- **Responsive Layout**: Fully responsive from desktop to mobile devices
- **Accessibility**: WCAG compliant with reduced motion support
- **Smooth Animations**: Parallax scrolling, fade-ins, and micro-interactions
- **Performance Optimized**: Efficient particle system and CSS animations

## Sections

1. **Hero Section**: Eye-catching animated background with bold typography
2. **Features Overview**: Showcase of AI Summary and Smart Discovery features
3. **Core Functions**: Detailed feature cards with hover effects
4. **Download Section**: Platform-specific download buttons for Windows, macOS, and Linux
5. **Footer**: Links, documentation, and social media connections

## Getting Started

### Prerequisites

- Node.js 16+ and npm/yarn/pnpm

### Installation

1. Install dependencies:
```bash
npm install
```

2. Start the development server:
```bash
npm run dev
```

3. Open your browser and navigate to `http://localhost:3000`

### Build for Production

```bash
npm run build
```

The built files will be in the `dist/` directory.

### Preview Production Build

```bash
npm run preview
```

## Project Structure

```
mavenrss-landing-page/
├── src/
│   ├── components/
│   │   ├── HeroSection.vue       # Hero with particle animation
│   │   ├── FeaturesOverview.vue  # Feature showcase
│   │   ├── CoreFunctions.vue     # Feature cards
│   │   ├── DownloadSection.vue   # Download buttons
│   │   └── Footer.vue            # Footer with links
│   ├── App.vue                   # Main app component
│   ├── main.js                   # Entry point
│   └── style.css                 # Global styles
├── index.html
├── package.json
├── vite.config.js
└── README.md
```

## Customization

### Colors

Edit the CSS variables in `src/style.css`:

```css
:root {
  --color-primary: #00d4ff;
  --color-secondary: #ff00ff;
  --color-tertiary: #ffff00;
  --color-bg-dark: #0a0a0f;
  /* ... */
}
```

### Content

Update the text and links in each component file:
- `HeroSection.vue`: Main headline and CTA
- `FeaturesOverview.vue`: Feature descriptions
- `CoreFunctions.vue`: Function highlights
- `DownloadSection.vue`: Download links
- `Footer.vue`: Navigation links and copyright

## Technologies Used

- **Vue.js 3**: Progressive JavaScript framework
- **Vite**: Next-generation build tool
- **CSS3**: Modern CSS with animations and gradients

## Browser Support

- Chrome/Edge (latest)
- Firefox (latest)
- Safari (latest)
- Mobile browsers (iOS Safari, Chrome Mobile)

## Performance

- Particle system optimized for 60fps
- CSS animations using GPU acceleration
- Lazy loading and code splitting ready
- Minimal bundle size with tree-shaking

## License

MIT

## Credits

Built with Vue.js and Vite
Design inspired by modernist aesthetics and brutalist web design
