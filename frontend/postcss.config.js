export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {
      // Ensure proper vendor prefix ordering
      cascade: true,
      grid: 'autoplace',
    },
  },
};
