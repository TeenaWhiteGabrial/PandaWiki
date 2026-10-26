(function() {
      function getBasename(pathname) {
        // 路由按特殊性从高到低排序（最具体的在前）
        var routes = [
          '/doc/editor/history/:id',
          '/doc/editor/space',
          '/doc/editor/:id',
          '/doc/editor',
          '/401',
          '/comment',
          '/conversation',
          '/login',
          '/release',
          '/setting',
          '/stat',
        ];

        // 将路由转为正则：静态段直接匹配，:param 匹配任意段，:param? 可选
        function toRegex(route) {
          var pattern = route.split('/').filter(Boolean).map(function(s) {
            if (s.startsWith(':')) return s.endsWith('?') ? '(?:/[^/]+)?' : '/[^/]+';
            return '/' + s;
          }).join('');
          return new RegExp('^(.*?)' + pattern + '/?$');
        }

        for (var i = 0; i < routes.length; i++) {
          var match = pathname.match(toRegex(routes[i]));
          if (match) return match[1].replace(/\/$/, '');
        }

        // 未匹配：移除最后一段作为 basename
        var parts = pathname.split('/').filter(Boolean);
        if (parts.length > 1) return '/' + parts.slice(0, -1).join('/');
        if (parts.length === 1 && pathname !== '/') return pathname.replace(/\/$/, '');
        return '';
      }
      var basename = getBasename(location.pathname);
      window.__BASENAME__ = basename;
      var favicon = document.getElementById('favicon');
      if (favicon) favicon.href = (basename || '') + '/logo.png';
    })();
