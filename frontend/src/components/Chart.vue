<template>
  <section class="chart">
    <canvas class="chart-canvas" />
  </section>
</template>

<script>
import Chart from 'chart.js/auto';

const chartText = '#cbd9ed';
const chartGrid = 'rgba(132, 165, 202, 0.22)';
const chartTooltip = '#0b1b35';

const darkTooltip = {
  backgroundColor: chartTooltip,
  borderColor: '#315a79',
  borderWidth: 1,
  titleColor: '#f3f8ff',
  bodyColor: chartText,
};

const DEFAULT_DONUT = {
  type: 'doughnut',
  data: {},
  options: {
    responsive: true,
    cutout: '70%',
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: false,
      },
      tooltip: {
        ...darkTooltip,
        bodyFont: {
          size: 15,
        },
        bodySpacing: 10,
        padding: 10,
        callbacks: {
          label: (item) => {
            const data = item.chart.data.datasets[item.datasetIndex];
            const total = data.data.reduce((acc, val) => acc + val, 0);
            const val = data.data[item.dataIndex];
            const percentage = ((val / total) * 100).toFixed(2);
            return `${val} (${percentage}%)`;
          },
        },
      },
    },
  },
};

const DEFAULT_LINE = {
  type: 'line',
  data: {},
  options: {
    responsive: true,
    lineTension: 0.5,
    maintainAspectRatio: false,
    interaction: {
      intersect: false,
      axis: 'index',
    },
    plugins: {
      legend: {
        display: false,
      },
      tooltip: {
        ...darkTooltip,
        displayColors: true,
        bodyFont: {
          size: 15,
        },
        bodySpacing: 10,
        padding: 10,
      },
    },
    scales: {
      x: {
        grid: {
          color: chartGrid,
        },
        ticks: {
          color: chartText,
        },
      },
      y: {
        grid: {
          color: chartGrid,
        },
        ticks: {
          color: chartText,
          precision: 0,
        },
      },
    },
  },
};

const DEFAULT_BAR = {
  type: 'bar',
  data: {},
  options: {
    responsive: true,
    indexAxis: 'y',
    barThickness: 40,
    maintainAspectRatio: false,
    plugins: {
      legend: {
        display: false,
      },
      tooltip: {
        ...darkTooltip,
        bodyFont: {
          size: 15,
        },
        bodySpacing: 10,
        padding: 10,
      },
    },
    scales: {
      x: {
        grid: {
          color: chartGrid,
        },
        ticks: {
          color: chartText,
        },
      },
      y: {
        grid: {
          color: chartGrid,
        },
        ticks: {
          color: chartText,
        },
      },
    },
  },
};

export default {
  name: 'Chart',

  props: {
    data: { type: Object, default: () => { } },
    type: { type: String, default: 'line' },
    onClick: { type: Function, default: () => { } },
  },

  mounted() {
    const ctx = this.$el.querySelector('.chart-canvas');

    let def = {};
    switch (this.$props.type) {
      case 'donut':
        def = DEFAULT_DONUT;
        break;
      case 'bar':
        def = DEFAULT_BAR;
        break;
      default:
        def = DEFAULT_LINE;
        break;
    }

    const conf = { ...def, data: this.$props.data };
    if (this.$props.onClick) {
      conf.options.onClick = this.$props.onClick;
    }
    this.chart = new Chart(ctx, conf);
  },
};
</script>
